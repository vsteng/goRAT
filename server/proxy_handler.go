package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mww2.com/server_manager/common"
)

// ProxyConnection represents a proxy tunnel connection
type ProxyConnection struct {
	ID           string
	ClientID     string
	LocalPort    int
	RemoteHost   string
	RemotePort   int
	Protocol     string // "tcp", "http", "https"
	Status       string // "active", "inactive", "error"
	BytesIn      int64
	BytesOut     int64
	CreatedAt    time.Time
	LastActive   time.Time
	listener     net.Listener
	mu           sync.RWMutex
	userChannels map[string]*net.Conn // Track user connections like lanproxy
	channelsMu   sync.RWMutex
}

// ProxyManager manages all proxy connections
type ProxyManager struct {
	connections map[string]*ProxyConnection
	mu          sync.RWMutex
	manager     *ClientManager
	store       *ClientStore   // For persistent storage
	portMap     map[int]string // Maps port to proxy connection ID (like lanproxy)
	portMapMu   sync.RWMutex
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(manager *ClientManager, store *ClientStore) *ProxyManager {
	return &ProxyManager{
		connections: make(map[string]*ProxyConnection),
		manager:     manager,
		store:       store,
		portMap:     make(map[int]string),
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

	// Verify websocket is open
	if client.Conn == nil || client.Conn.RemoteAddr() == nil {
		return nil, fmt.Errorf("client websocket not connected: %s", clientID)
	}

	// Normalize protocol to lowercase
	protocol = strings.ToLower(protocol)
	if protocol == "" {
		protocol = "tcp"
	}

	// Generate unique ID
	id := fmt.Sprintf("%s-%d-%d", clientID, localPort, time.Now().Unix())

	conn := &ProxyConnection{
		ID:           id,
		ClientID:     clientID,
		LocalPort:    localPort,
		RemoteHost:   remoteHost,
		RemotePort:   remotePort,
		Protocol:     protocol,
		Status:       "starting",
		CreatedAt:    time.Now(),
		LastActive:   time.Now(),
		userChannels: make(map[string]*net.Conn),
	}

	// Start listening on local port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %v", localPort, err)
	}

	conn.listener = listener
	conn.Status = "active"

	// Register port mapping
	pm.portMapMu.Lock()
	pm.portMap[localPort] = id
	pm.portMapMu.Unlock()

	// Store connection
	pm.connections[id] = conn

	// Persist to database if store is available
	if pm.store != nil {
		if err := pm.store.SaveProxy(conn); err != nil {
			log.Printf("WARNING: Failed to save proxy to database: %v", err)
		}
	}

	// Start accepting connections
	go pm.acceptConnections(conn)

	log.Printf("Created proxy connection: %s (client: %s, local: :%d -> %s:%d protocol: %s)",
		id, clientID, localPort, remoteHost, remotePort, protocol)

	return conn, nil
}

// acceptConnections accepts incoming connections on the proxy listener
func (pm *ProxyManager) acceptConnections(conn *ProxyConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in acceptConnections: %v", r)
			conn.Status = "error"
		}
	}()

	for {
		conn.mu.RLock()
		if conn.Status != "active" {
			conn.mu.RUnlock()
			break
		}
		conn.mu.RUnlock()

		if conn.listener == nil {
			break
		}

		// Set deadline to allow checking status periodically
		if tcpListener, ok := conn.listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(5 * time.Second))
		}

		userConn, err := conn.listener.Accept()
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

		// Generate user ID for this connection
		userID := fmt.Sprintf("user-%d-%d", conn.LocalPort, time.Now().UnixNano())

		// Store the user connection
		conn.channelsMu.Lock()
		conn.userChannels[userID] = &userConn
		conn.channelsMu.Unlock()

		log.Printf("New user connection accepted on proxy %s: %s", conn.ID, userID)

		// Handle the connection in a goroutine
		go pm.handleUserConnection(conn, userConn, userID)
	}

	conn.mu.Lock()
	if conn.listener != nil {
		conn.listener.Close()
		conn.listener = nil
	}
	conn.mu.Unlock()

	log.Printf("Stopped accepting connections for proxy %s", conn.ID)
}

// sendWebSocketMessage sends a message to websocket with timeout (non-blocking)
func (pm *ProxyManager) sendWebSocketMessage(conn interface{}, msg interface{}) error {
	// Type assert to get the actual websocket connection
	wsConn, ok := conn.(*websocket.Conn)
	if !ok || wsConn == nil {
		return fmt.Errorf("websocket connection is invalid")
	}

	// Use a channel to track write completion with timeout
	done := make(chan error, 1)
	go func() {
		done <- wsConn.WriteJSON(msg)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("websocket write timeout")
	}
}

// handleUserConnection handles a user connection by relaying through websocket to the remote server
func (pm *ProxyManager) handleUserConnection(proxyConn *ProxyConnection, userConn net.Conn, userID string) {
	defer func() {
		userConn.Close()

		// Remove from tracking
		proxyConn.channelsMu.Lock()
		delete(proxyConn.userChannels, userID)
		proxyConn.channelsMu.Unlock()

		// Notify client of disconnect (best effort, async)
		client, ok := pm.manager.GetClient(proxyConn.ClientID)
		if ok && client.Conn != nil {
			msg := map[string]interface{}{
				"type":     "proxy_disconnect",
				"proxy_id": proxyConn.ID,
				"user_id":  userID,
			}
			go pm.sendWebSocketMessage(client.Conn, msg) // Async, don't block
		}

		log.Printf("User connection closed: proxy=%s, user=%s", proxyConn.ID, userID)
	}()

	// Get the client
	client, ok := pm.manager.GetClient(proxyConn.ClientID)
	if !ok {
		log.Printf("Client %s not found for proxy relay", proxyConn.ClientID)
		return
	}

	if client.Conn == nil {
		log.Printf("Client websocket not connected: %s", proxyConn.ClientID)
		return
	}

	// Send connect request to client with timeout
	connectMsg := map[string]interface{}{
		"type":        "proxy_connect",
		"proxy_id":    proxyConn.ID,
		"user_id":     userID,
		"remote_host": proxyConn.RemoteHost,
		"remote_port": proxyConn.RemotePort,
		"protocol":    proxyConn.Protocol,
	}

	if err := pm.sendWebSocketMessage(client.Conn, connectMsg); err != nil {
		log.Printf("Failed to send proxy_connect message: %v", err)
		return
	}

	log.Printf("Sent proxy_connect to client: proxy=%s, user=%s, remote=%s:%d",
		proxyConn.ID, userID, proxyConn.RemoteHost, proxyConn.RemotePort)

	// Read from user connection and relay to client via websocket
	buf := make([]byte, 4096)
	for {
		userConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := userConn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from user connection: %v", err)
			}
			break
		}

		if n > 0 {
			proxyConn.mu.Lock()
			proxyConn.BytesIn += int64(n)
			proxyConn.LastActive = time.Now()
			proxyConn.mu.Unlock()

			// Send data to client via websocket (encode binary data as base64)
			dataMsg := map[string]interface{}{
				"type":     "proxy_data",
				"proxy_id": proxyConn.ID,
				"user_id":  userID,
				"data":     base64.StdEncoding.EncodeToString(buf[:n]),
			}

			if err := pm.sendWebSocketMessage(client.Conn, dataMsg); err != nil {
				log.Printf("Failed to send proxy_data message: %v", err)
				break
			}
		}
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

	// Clean up port mapping
	pm.portMapMu.Lock()
	delete(pm.portMap, conn.LocalPort)
	pm.portMapMu.Unlock()

	// Close all user connections
	conn.channelsMu.Lock()
	for _, userConnPtr := range conn.userChannels {
		if userConnPtr != nil && *userConnPtr != nil {
			(*userConnPtr).Close()
		}
	}
	conn.userChannels = make(map[string]*net.Conn)
	conn.channelsMu.Unlock()

	delete(pm.connections, id)
	log.Printf("Closed proxy connection: %s (port %d)", id, conn.LocalPort)

	// Update database status if store is available
	if pm.store != nil {
		if err := pm.store.DeleteProxy(id); err != nil {
			log.Printf("WARNING: Failed to delete proxy from database: %v", err)
		}
	}

	return nil
}

// HandleProxyDataFromClient processes incoming proxy data from a client websocket
func (pm *ProxyManager) HandleProxyDataFromClient(proxyID, userID string, data []byte) error {
	pm.mu.RLock()
	conn, exists := pm.connections[proxyID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("proxy connection not found: %s", proxyID)
	}

	conn.channelsMu.RLock()
	userConnPtr, userExists := conn.userChannels[userID]
	conn.channelsMu.RUnlock()

	if !userExists || userConnPtr == nil || *userConnPtr == nil {
		return fmt.Errorf("user connection not found: proxy=%s, user=%s", proxyID, userID)
	}

	// Write data to user connection
	userConn := *userConnPtr
	n, err := userConn.Write(data)
	if err != nil {
		log.Printf("Error writing to user connection: %v", err)
		return err
	}

	conn.mu.Lock()
	conn.BytesOut += int64(n)
	conn.LastActive = time.Now()
	conn.mu.Unlock()

	return nil
}

// GetProxyConnection retrieves a proxy connection by ID
func (pm *ProxyManager) GetProxyConnection(id string) *ProxyConnection {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.connections[id]
}

// UpdateProxyConnection updates an existing proxy connection settings
func (pm *ProxyManager) UpdateProxyConnection(id, remoteHost string, remotePort, localPort int, protocol string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	conn, exists := pm.connections[id]
	if !exists {
		return fmt.Errorf("proxy connection not found: %s", id)
	}

	// Normalize protocol to lowercase
	protocol = strings.ToLower(protocol)
	if protocol == "" {
		protocol = "tcp"
	}

	// If port changed, update port mapping
	if localPort != conn.LocalPort {
		// Check if new port is available
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
		if err != nil {
			return fmt.Errorf("failed to listen on new port %d: %v", localPort, err)
		}

		// Close old listener
		if conn.listener != nil {
			conn.listener.Close()
		}

		// Update port mapping
		pm.portMapMu.Lock()
		delete(pm.portMap, conn.LocalPort)
		pm.portMap[localPort] = id
		pm.portMapMu.Unlock()

		// Update listener
		conn.listener = listener
		conn.LocalPort = localPort

		// Restart accepting connections with new listener
		go pm.acceptConnections(conn)
	}

	// Update other fields
	conn.mu.Lock()
	conn.RemoteHost = remoteHost
	conn.RemotePort = remotePort
	conn.Protocol = protocol
	conn.LastActive = time.Now()
	conn.mu.Unlock()

	// Persist changes to database
	if pm.store != nil {
		if err := pm.store.UpdateProxy(conn); err != nil {
			log.Printf("WARNING: Failed to update proxy in database: %v", err)
		}
	}

	log.Printf("Updated proxy connection: %s (local: :%d -> %s:%d, protocol: %s)",
		id, localPort, remoteHost, remotePort, protocol)

	return nil
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

	var rawReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Extract fields, supporting both snake_case and camelCase
	clientID := ""
	remoteHost := ""
	remotePort := 0
	localPort := 0
	protocol := ""

	// Try snake_case first
	if v, ok := rawReq["client_id"].(string); ok {
		clientID = v
	} else if v, ok := rawReq["clientId"].(string); ok {
		clientID = v
	}

	if v, ok := rawReq["remote_host"].(string); ok {
		remoteHost = v
	} else if v, ok := rawReq["remoteHost"].(string); ok {
		remoteHost = v
	}

	if v, ok := rawReq["remote_port"].(float64); ok {
		remotePort = int(v)
	} else if v, ok := rawReq["remotePort"].(float64); ok {
		remotePort = int(v)
	}

	if v, ok := rawReq["local_port"].(float64); ok {
		localPort = int(v)
	} else if v, ok := rawReq["localPort"].(float64); ok {
		localPort = int(v)
	}

	if v, ok := rawReq["protocol"].(string); ok {
		protocol = v
	}

	// Validate required fields
	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}
	if remoteHost == "" {
		http.Error(w, "Missing remote_host", http.StatusBadRequest)
		return
	}
	if remotePort == 0 {
		http.Error(w, "Missing remote_port", http.StatusBadRequest)
		return
	}
	if localPort == 0 {
		http.Error(w, "Missing local_port", http.StatusBadRequest)
		return
	}

	if protocol == "" {
		protocol = "tcp"
	}

	// Create proxy manager if not exists
	if s.proxyManager == nil {
		s.proxyManager = NewProxyManager(s.manager, s.store)
	}

	conn, err := s.proxyManager.CreateProxyConnection(
		clientID, remoteHost, remotePort, localPort, protocol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conn)
}

// HandleProxyList lists proxy connections via HTTP
func (s *Server) HandleProxyList(w http.ResponseWriter, r *http.Request) {
	// Support both snake_case and camelCase
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		clientID = r.URL.Query().Get("client_id")
	}

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

// HandleProxyEdit updates an existing proxy connection via HTTP
func (s *Server) HandleProxyEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rawReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Extract fields
	proxyID := ""
	remoteHost := ""
	remotePort := 0
	localPort := 0
	protocol := ""

	// Try snake_case and camelCase
	if v, ok := rawReq["proxy_id"].(string); ok {
		proxyID = v
	} else if v, ok := rawReq["proxyId"].(string); ok {
		proxyID = v
	}

	if v, ok := rawReq["remote_host"].(string); ok {
		remoteHost = v
	} else if v, ok := rawReq["remoteHost"].(string); ok {
		remoteHost = v
	}

	if v, ok := rawReq["remote_port"].(float64); ok {
		remotePort = int(v)
	} else if v, ok := rawReq["remotePort"].(float64); ok {
		remotePort = int(v)
	}

	if v, ok := rawReq["local_port"].(float64); ok {
		localPort = int(v)
	} else if v, ok := rawReq["localPort"].(float64); ok {
		localPort = int(v)
	}

	if v, ok := rawReq["protocol"].(string); ok {
		protocol = v
	}

	// Validate required fields
	if proxyID == "" {
		http.Error(w, "Missing proxy_id", http.StatusBadRequest)
		return
	}
	if remoteHost == "" {
		http.Error(w, "Missing remote_host", http.StatusBadRequest)
		return
	}
	if remotePort == 0 {
		http.Error(w, "Missing remote_port", http.StatusBadRequest)
		return
	}
	if localPort == 0 {
		http.Error(w, "Missing local_port", http.StatusBadRequest)
		return
	}

	if protocol == "" {
		protocol = "tcp"
	}

	if s.proxyManager == nil {
		http.Error(w, "Proxy manager not initialized", http.StatusInternalServerError)
		return
	}

	if err := s.proxyManager.UpdateProxyConnection(proxyID, remoteHost, remotePort, localPort, protocol); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn := s.proxyManager.GetProxyConnection(proxyID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conn)
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
	var meta *common.ClientMetadata
	if client.Metadata == nil {
		// Fallback: return minimal metadata if nil
		meta = &common.ClientMetadata{
			ID:     client.ID,
			Status: "unknown",
		}
	} else {
		meta = client.Metadata
		// Ensure ID is populated from client struct if missing
		if meta.ID == "" {
			meta.ID = client.ID
		}
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

	// Clear any previous result
	s.ClearProcessListResult(clientID)

	// Send process list request to client
	msg, err := common.NewMessage(common.MsgTypeListProcesses, nil)
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := s.manager.SendToClient(clientID, msg); err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	// Wait for response with timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			http.Error(w, "Request timeout", http.StatusRequestTimeout)
			return
		case <-ticker.C:
			if result, exists := s.GetProcessListResult(clientID); exists {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result.Processes)
				s.ClearProcessListResult(clientID)
				return
			}
		}
	}
}
