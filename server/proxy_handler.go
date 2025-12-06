package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// ProxyConnection represents a proxy tunnel connection
type ProxyConnection struct {
	ID         string
	ClientID   string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Protocol   string // "tcp", "http", "https"
	Status     string // "active", "inactive", "error"
	BytesIn    int64
	BytesOut   int64
	CreatedAt  time.Time
	LastActive time.Time
	listener   net.Listener
	mu         sync.RWMutex
}

// ProxyManager manages all proxy connections
type ProxyManager struct {
	connections map[string]*ProxyConnection
	mu          sync.RWMutex
	manager     *ClientManager
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(manager *ClientManager) *ProxyManager {
	return &ProxyManager{
		connections: make(map[string]*ProxyConnection),
		manager:     manager,
	}
}

// CreateProxyConnection creates a new proxy tunnel
func (pm *ProxyManager) CreateProxyConnection(clientID, remoteHost string, remotePort, localPort int, protocol string) (*ProxyConnection, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Verify client is connected
	client, exists := pm.manager.GetClient(clientID)
	if !exists {
		return nil, fmt.Errorf("client not found: %s", clientID)
	}
	_ = client // Use client to avoid unused variable warning

	// Generate unique ID
	id := fmt.Sprintf("%s-%d-%d", clientID, localPort, time.Now().Unix())

	conn := &ProxyConnection{
		ID:         id,
		ClientID:   clientID,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Protocol:   protocol,
		Status:     "starting",
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	// Start listening on local port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %v", localPort, err)
	}

	conn.listener = listener
	conn.Status = "active"

	// Start accepting connections
	go pm.acceptConnections(conn)

	pm.connections[id] = conn
	log.Printf("Created proxy connection: %s (client: %s, local: %d -> remote: %s:%d)",
		id, clientID, localPort, remoteHost, remotePort)

	return conn, nil
}

// acceptConnections accepts incoming connections on the proxy listener
func (pm *ProxyManager) acceptConnections(conn *ProxyConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in acceptConnections: %v", r)
		}
	}()

	for {
		conn.mu.RLock()
		if conn.Status != "active" {
			conn.mu.RUnlock()
			break
		}
		conn.mu.RUnlock()

		listener := conn.listener
		if listener == nil {
			break
		}

		// Set deadline to allow checking status periodically
		listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))

		clientConn, err := listener.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			if conn.Status != "active" {
				break
			}
			log.Printf("Error accepting connection on proxy %s: %v", conn.ID, err)
			continue
		}

		// Relay connection to client through websocket
		go pm.relayConnection(conn, clientConn)
	}

	conn.mu.Lock()
	if conn.listener != nil {
		conn.listener.Close()
		conn.listener = nil
	}
	conn.mu.Unlock()
}

// relayConnection relays a connection through the client
func (pm *ProxyManager) relayConnection(proxyConn *ProxyConnection, clientConn net.Conn) {
	defer clientConn.Close()

	client, exists := pm.manager.GetClient(proxyConn.ClientID)
	if !exists {
		log.Printf("Client %s not found for proxy relay", proxyConn.ClientID)
		return
	}
	_ = client

	// Send proxy request to client
	msg := map[string]interface{}{
		"type":        "proxy_request",
		"proxy_id":    proxyConn.ID,
		"remote_host": proxyConn.RemoteHost,
		"remote_port": proxyConn.RemotePort,
		"protocol":    proxyConn.Protocol,
	}

	data, _ := json.Marshal(msg)

	// TODO: Send through existing websocket connection
	// For now, this is a placeholder for the actual relay logic

	// Read from client connection and send to remote
	buf := make([]byte, 4096)
	for {
		n, err := clientConn.Read(buf)
		if err != nil {
			break
		}

		proxyConn.mu.Lock()
		proxyConn.BytesIn += int64(n)
		proxyConn.LastActive = time.Now()
		proxyConn.mu.Unlock()

		// TODO: Forward to client via websocket
		_ = data
	}
}

// CloseProxyConnection closes a proxy connection
func (pm *ProxyManager) CloseProxyConnection(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	conn, exists := pm.connections[id]
	if !exists {
		return fmt.Errorf("proxy connection not found: %s", id)
	}

	conn.mu.Lock()
	conn.Status = "inactive"
	if conn.listener != nil {
		conn.listener.Close()
		conn.listener = nil
	}
	conn.mu.Unlock()

	delete(pm.connections, id)
	log.Printf("Closed proxy connection: %s", id)

	return nil
}

// GetProxyConnection retrieves a proxy connection by ID
func (pm *ProxyManager) GetProxyConnection(id string) *ProxyConnection {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.connections[id]
}

// ListProxyConnections lists all proxy connections for a client
func (pm *ProxyManager) ListProxyConnections(clientID string) []*ProxyConnection {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*ProxyConnection
	for _, conn := range pm.connections {
		if conn.ClientID == clientID {
			result = append(result, conn)
		}
	}
	return result
}

// ListAllProxyConnections lists all proxy connections
func (pm *ProxyManager) ListAllProxyConnections() []*ProxyConnection {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*ProxyConnection
	for _, conn := range pm.connections {
		result = append(result, conn)
	}
	return result
}

// HandleProxyCreate handles creating a new proxy connection via HTTP
func (s *Server) HandleProxyCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID   string `json:"client_id"`
		RemoteHost string `json:"remote_host"`
		RemotePort int    `json:"remote_port"`
		LocalPort  int    `json:"local_port"`
		Protocol   string `json:"protocol"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Protocol == "" {
		req.Protocol = "tcp"
	}

	// Create proxy manager if not exists
	if s.proxyManager == nil {
		s.proxyManager = NewProxyManager(s.manager)
	}

	conn, err := s.proxyManager.CreateProxyConnection(
		req.ClientID, req.RemoteHost, req.RemotePort, req.LocalPort, req.Protocol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conn)
}

// HandleProxyList lists proxy connections via HTTP
func (s *Server) HandleProxyList(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")

	if s.proxyManager == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ProxyConnection{})
		return
	}

	var connections []*ProxyConnection
	if clientID != "" {
		connections = s.proxyManager.ListProxyConnections(clientID)
	} else {
		connections = s.proxyManager.ListAllProxyConnections()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connections)
}

// HandleProxyClose closes a proxy connection via HTTP
func (s *Server) HandleProxyClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing proxy ID", http.StatusBadRequest)
		return
	}

	if s.proxyManager == nil {
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	if err := s.proxyManager.CloseProxyConnection(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
}

// HandleProxyStats retrieves statistics for a proxy connection
func (s *Server) HandleProxyStats(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing proxy ID", http.StatusBadRequest)
		return
	}

	if s.proxyManager == nil {
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	conn := s.proxyManager.GetProxyConnection(id)
	if conn == nil {
		http.Error(w, "Proxy connection not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conn)
}

// ProxyFileServer serves files through proxy connections (similar to LanProxy)
func (s *Server) ProxyFileServer(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	path := r.URL.Query().Get("path")

	if clientID == "" || path == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	client, exists := s.manager.GetClient(clientID)
	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
	_ = client

	// Send file request to client
	msg := map[string]interface{}{
		"type": "download_file",
		"path": path,
	}

	data, _ := json.Marshal(msg)
	_ = data // TODO: Send through websocket and receive file data

	// Placeholder: Return file data
	w.Header().Set("Content-Type", "application/octet-stream")
	io.WriteString(w, "File content placeholder")
}

// HandleClientGet retrieves a specific client by ID
func (s *Server) HandleClientGet(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		// Try from path parameter
		clientID = r.PathValue("id")
	}

	if clientID == "" {
		http.Error(w, "Missing client ID", http.StatusBadRequest)
		return
	}

	client, exists := s.manager.GetClient(clientID)
	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Prefer returning metadata only to avoid marshaling websocket.Conn and other internal fields.
	if client.Metadata == nil {
		http.Error(w, "Client metadata unavailable", http.StatusInternalServerError)
		return
	}

	meta := *client.Metadata
	// Ensure ID is populated from client struct if missing
	if meta.ID == "" {
		meta.ID = client.ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// HandleFilesAPI serves file list for a client
func (s *Server) HandleFilesAPI(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	path := r.URL.Query().Get("path")

	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}

	if path == "" {
		path = "/"
	}

	client, exists := s.manager.GetClient(clientID)
	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
	_ = client

	// Send file list request to client
	msg := map[string]interface{}{
		"type": "browse_files",
		"path": path,
	}

	data, _ := json.Marshal(msg)

	// TODO: Send request through websocket and wait for response
	// For now, return mock data
	_ = data

	mockFiles := []map[string]interface{}{
		{
			"name":     "file1.txt",
			"path":     "/file1.txt",
			"size":     1024,
			"modified": time.Now().Unix(),
			"is_dir":   false,
		},
		{
			"name":     "folder1",
			"path":     "/folder1",
			"size":     0,
			"modified": time.Now().Unix(),
			"is_dir":   true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mockFiles)
}

// HandleProcessesAPI serves process list for a client
func (s *Server) HandleProcessesAPI(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")

	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}

	client, exists := s.manager.GetClient(clientID)
	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
	_ = client

	// Send process list request to client
	msg := map[string]interface{}{
		"type": "list_processes",
	}

	data, _ := json.Marshal(msg)
	_ = data // TODO: Send through websocket

	// Return mock data
	mockProcesses := []map[string]interface{}{
		{
			"name":   "svchost.exe",
			"pid":    4,
			"cpu":    2.5,
			"memory": 15.3,
			"status": "running",
		},
		{
			"name":   "explorer.exe",
			"pid":    1024,
			"cpu":    5.1,
			"memory": 45.6,
			"status": "running",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mockProcesses)
}
