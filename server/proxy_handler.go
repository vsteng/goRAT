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

	"gorat/pkg/clients"
	"gorat/pkg/protocol"
	"gorat/pkg/proxy"
	"gorat/pkg/storage"

	"github.com/gorilla/websocket"
)

// ProxyConnection represents a proxy tunnel connection
type ProxyConnection struct {
	ID           string
	ClientID     string
	LocalPort    int
	RemoteHost   string
	RemotePort   int
	Protocol     string // "tcp", "http", "https"
	BytesIn      int64
	BytesOut     int64
	CreatedAt    time.Time
	LastActive   time.Time
	listener     net.Listener
	mu           sync.RWMutex
	userChannels map[string]*net.Conn // Track user connections like lanproxy
	channelsMu   sync.RWMutex
	MaxIdleTime  time.Duration   // Auto-close if idle for this duration (0 = never)
	UserCount    int             // Current number of active user connections
	connPool     *ConnectionPool // Connection pool for reusing client connections
}

// PooledConnection represents a reusable connection to the remote target
type PooledConnection struct {
	conn       net.Conn
	lastUsed   time.Time
	inUse      bool
	created    time.Time
	usageCount int
}

// ConnectionPool manages a pool of connections to remote targets
type ConnectionPool struct {
	mu          sync.RWMutex
	connections []*PooledConnection
	maxSize     int
	maxIdleTime time.Duration
	maxLifetime time.Duration
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(maxSize int, maxIdleTime, maxLifetime time.Duration) *ConnectionPool {
	return &ConnectionPool{
		connections: make([]*PooledConnection, 0, maxSize),
		maxSize:     maxSize,
		maxIdleTime: maxIdleTime,
		maxLifetime: maxLifetime,
	}
}

// Get retrieves a connection from the pool or creates a new one
func (cp *ConnectionPool) Get(remoteHost string, remotePort int) (net.Conn, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()

	// Try to find an available connection
	for i, pc := range cp.connections {
		if pc.inUse {
			continue
		}

		// Check if connection is still valid
		idleTime := now.Sub(pc.lastUsed)
		lifetime := now.Sub(pc.created)

		// Remove stale connections
		if idleTime > cp.maxIdleTime || lifetime > cp.maxLifetime {
			pc.conn.Close()
			cp.connections = append(cp.connections[:i], cp.connections[i+1:]...)
			continue
		}

		// Test if connection is still alive
		pc.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		one := make([]byte, 1)
		_, err := pc.conn.Read(one)
		pc.conn.SetReadDeadline(time.Time{}) // Clear deadline

		if err == nil {
			// Connection has data, put it back and skip
			continue
		}

		// Connection is good (timeout or EOF is expected for idle connection)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			pc.inUse = true
			pc.lastUsed = now
			pc.usageCount++
			return pc.conn, nil
		}
	}

	// No available connection, create new one if under limit
	if len(cp.connections) < cp.maxSize {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort), 10*time.Second)
		if err != nil {
			return nil, err
		}

		pc := &PooledConnection{
			conn:       conn,
			lastUsed:   now,
			inUse:      true,
			created:    now,
			usageCount: 1,
		}
		cp.connections = append(cp.connections, pc)
		return conn, nil
	}

	// Pool is full, create a temporary connection (not pooled)
	return net.DialTimeout("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort), 10*time.Second)
}

// Put returns a connection to the pool
func (cp *ConnectionPool) Put(conn net.Conn) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, pc := range cp.connections {
		if pc.conn == conn {
			pc.inUse = false
			pc.lastUsed = time.Now()
			return
		}
	}

	// Connection not in pool, just close it
	conn.Close()
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, pc := range cp.connections {
		pc.conn.Close()
	}
	cp.connections = nil
}

// CleanIdle removes idle connections that exceeded max idle time
func (cp *ConnectionPool) CleanIdle() int {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()
	removed := 0
	newConns := make([]*PooledConnection, 0, len(cp.connections))

	for _, pc := range cp.connections {
		if pc.inUse {
			newConns = append(newConns, pc)
			continue
		}

		idleTime := now.Sub(pc.lastUsed)
		lifetime := now.Sub(pc.created)

		if idleTime > cp.maxIdleTime || lifetime > cp.maxLifetime {
			pc.conn.Close()
			removed++
		} else {
			newConns = append(newConns, pc)
		}
	}

	cp.connections = newConns
	return removed
}

// Stats returns pool statistics
func (cp *ConnectionPool) Stats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	total := len(cp.connections)
	inUse := 0
	for _, pc := range cp.connections {
		if pc.inUse {
			inUse++
		}
	}

	return map[string]interface{}{
		"total":     total,
		"in_use":    inUse,
		"available": total - inUse,
		"max_size":  cp.maxSize,
	}
}

// ProxyManager manages all proxy connections
type ProxyManager struct {
	connections map[string]*ProxyConnection
	mu          sync.RWMutex
	manager     clients.Manager
	store       storage.Store  // For persistent storage
	portMap     map[int]string // Maps port to proxy connection ID (like lanproxy)
	portMapMu   sync.RWMutex
	stopMonitor chan struct{} // Signal to stop idle monitoring
	wsLocks     sync.Map      // per-client websocket write locks for raw proxy frames
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(manager clients.Manager, store storage.Store) *ProxyManager {
	pm := &ProxyManager{
		connections: make(map[string]*ProxyConnection),
		manager:     manager,
		store:       store,
		portMap:     make(map[int]string),
		stopMonitor: make(chan struct{}),
	}

	// Start idle connection monitor
	go pm.monitorIdleConnections()

	return pm
}

// toStorageProxy converts a ProxyConnection to storage.ProxyConnection for persistence
func (conn *ProxyConnection) toStorageProxy() *storage.ProxyConnection {
	return &storage.ProxyConnection{
		ID:          conn.ID,
		ClientID:    conn.ClientID,
		LocalPort:   conn.LocalPort,
		RemoteHost:  conn.RemoteHost,
		RemotePort:  conn.RemotePort,
		Protocol:    conn.Protocol,
		BytesIn:     conn.BytesIn,
		BytesOut:    conn.BytesOut,
		CreatedAt:   conn.CreatedAt,
		LastActive:  conn.LastActive,
		UserCount:   conn.UserCount,
		MaxIdleTime: conn.MaxIdleTime,
	}
}

// FindAvailablePort finds an available port starting from the suggested port
func (pm *ProxyManager) FindAvailablePort(suggestedPort int) (int, error) {
	pm.portMapMu.RLock()
	defer pm.portMapMu.RUnlock()

	// Check if suggested port is available
	if _, exists := pm.portMap[suggestedPort]; !exists {
		// Try to bind to verify it's truly available
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", suggestedPort))
		if err == nil {
			listener.Close()
			return suggestedPort, nil
		}
	}

	// If suggested port is taken, find next available
	for port := suggestedPort + 1; port < suggestedPort+100; port++ {
		if _, exists := pm.portMap[port]; !exists {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				listener.Close()
				return port, nil
			}
		}
	}

	return 0, fmt.Errorf("no available ports found in range %d-%d", suggestedPort, suggestedPort+99)
}

// GetSuggestedPorts returns a list of suggested available ports
func (pm *ProxyManager) GetSuggestedPorts(basePort int, count int) []int {
	pm.portMapMu.RLock()
	defer pm.portMapMu.RUnlock()

	var suggested []int
	for port := basePort; len(suggested) < count && port < basePort+1000; port++ {
		if _, exists := pm.portMap[port]; !exists {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				listener.Close()
				suggested = append(suggested, port)
			}
		}
	}
	return suggested
}

// CreateProxyConnection creates a new proxy tunnel
func (pm *ProxyManager) CreateProxyConnection(clientID, remoteHost string, remotePort, localPort int, protocol string) (*ProxyConnection, error) {
	return pm.createProxyConnectionWithID("", clientID, remoteHost, remotePort, localPort, protocol)
}

// createProxyConnectionWithID creates a proxy with an optional specific ID (used for restores)
func (pm *ProxyManager) createProxyConnectionWithID(id, clientID, remoteHost string, remotePort, localPort int, protocol string) (*ProxyConnection, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Verify client is connected
	client, exists := pm.manager.GetClient(clientID)
	if !exists {
		return nil, fmt.Errorf("client not found: %s", clientID)
	}

	// Verify websocket is open
	wsConn := client.Conn()
	if wsConn == nil {
		return nil, fmt.Errorf("client websocket not connected: %s", clientID)
	}

	// Check for port conflict
	pm.portMapMu.RLock()
	if existingID, exists := pm.portMap[localPort]; exists {
		pm.portMapMu.RUnlock()
		return nil, fmt.Errorf("port %d is already in use by proxy %s", localPort, existingID)
	}
	pm.portMapMu.RUnlock()

	// Normalize protocol to lowercase
	protocol = strings.ToLower(protocol)
	if protocol == "" {
		protocol = "tcp"
	}

	// Generate unique ID if not provided
	if id == "" {
		id = fmt.Sprintf("%s-%d-%d", clientID, localPort, time.Now().Unix())
	}

	conn := &ProxyConnection{
		ID:           id,
		ClientID:     clientID,
		LocalPort:    localPort,
		RemoteHost:   remoteHost,
		RemotePort:   remotePort,
		Protocol:     protocol,
		CreatedAt:    time.Now(),
		LastActive:   time.Now(),
		userChannels: make(map[string]*net.Conn),
		MaxIdleTime:  0, // 0 = never auto-close (can be configured per proxy)
		UserCount:    0,
		connPool:     NewConnectionPool(10, 5*time.Minute, 30*time.Minute), // Pool: max 10 conns, 5min idle, 30min lifetime
	}

	// Start listening on local port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %v", localPort, err)
	}

	conn.listener = listener

	// Register port mapping
	pm.portMapMu.Lock()
	pm.portMap[localPort] = id
	pm.portMapMu.Unlock()

	// Store connection
	pm.connections[id] = conn

	// Persist to database if store is available
	if pm.store != nil {
		if err := pm.store.SaveProxy(conn.toStorageProxy()); err != nil {
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
		}
	}()

	for {
		// Check if listener is still active
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
			if conn.listener == nil {
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
		conn.UserCount++
		conn.channelsMu.Unlock()

		log.Printf("New user connection accepted on proxy %s: %s (total users: %d)", conn.ID, userID, conn.UserCount)

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

// sendWebSocketMessage sends a message to websocket (thread-safe write)
func (pm *ProxyManager) getClientLock(clientID string) *sync.Mutex {
	if v, ok := pm.wsLocks.Load(clientID); ok {
		return v.(*sync.Mutex)
	}
	mu := &sync.Mutex{}
	actual, _ := pm.wsLocks.LoadOrStore(clientID, mu)
	return actual.(*sync.Mutex)
}

// sendWebSocketMessage delivers either protocol messages (via client channel) or raw proxy frames (map) using the client's write lock.
func (pm *ProxyManager) sendWebSocketMessage(client clients.Client, msg interface{}) error {
	if client == nil {
		return fmt.Errorf("client is invalid")
	}

	switch m := msg.(type) {
	case *protocol.Message:
		return client.SendMessage(m)
	case map[string]interface{}:
		lock := pm.getClientLock(client.ID())
		lock.Lock()
		defer lock.Unlock()
		return client.SendRaw(func(conn *websocket.Conn) error {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			return conn.WriteJSON(m)
		})
	default:
		return fmt.Errorf("unsupported message type for proxy communication")
	}
}

// handleUserConnection handles a user connection by relaying through websocket to the remote server
func (pm *ProxyManager) handleUserConnection(proxyConn *ProxyConnection, userConn net.Conn, userID string) {
	defer func() {
		userConn.Close()

		// Remove from tracking
		proxyConn.channelsMu.Lock()
		delete(proxyConn.userChannels, userID)
		proxyConn.UserCount--
		proxyConn.channelsMu.Unlock()

		// Notify client of disconnect (best effort, async)
		client, ok := pm.manager.GetClient(proxyConn.ClientID)
		if ok && client.Conn() != nil {
			msg := map[string]interface{}{
				"type":     "proxy_disconnect",
				"proxy_id": proxyConn.ID,
				"user_id":  userID,
			}
			go pm.sendWebSocketMessage(client, msg) // Async, don't block
		}

		log.Printf("User connection closed: proxy=%s, user=%s (remaining users: %d)", proxyConn.ID, userID, proxyConn.UserCount)
	}()

	// Get the client
	client, ok := pm.manager.GetClient(proxyConn.ClientID)
	if !ok {
		log.Printf("Client %s not found for proxy relay", proxyConn.ClientID)
		return
	}

	if client.Conn() == nil {
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

	if err := pm.sendWebSocketMessage(client, connectMsg); err != nil {
		log.Printf("Failed to send proxy_connect message: %v", err)
		return
	}

	log.Printf("Sent proxy_connect to client: proxy=%s, user=%s, remote=%s:%d",
		proxyConn.ID, userID, proxyConn.RemoteHost, proxyConn.RemotePort)

	// Read from user connection and relay to client via websocket
	// Increased buffer size for better throughput (16KB like LanProxy's typical frame size)
	buf := make([]byte, 16384)

	// Batching mechanism to reduce WebSocket message overhead
	batchTimeout := time.NewTimer(5 * time.Millisecond)
	defer batchTimeout.Stop()

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

			if err := pm.sendWebSocketMessage(client, dataMsg); err != nil {
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

	// Close connection pool
	if conn.connPool != nil {
		conn.connPool.Close()
	}

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

// HandleProxyDisconnect handles a user disconnecting from a proxy tunnel
func (pm *ProxyManager) HandleProxyDisconnect(proxyID, userID string) error {
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
		// Already closed or doesn't exist - that's fine
		return nil
	}

	// Close the user's connection
	userConn := *userConnPtr
	if userConn != nil {
		userConn.Close()
	}

	// Remove from tracking
	conn.channelsMu.Lock()
	delete(conn.userChannels, userID)
	conn.channelsMu.Unlock()

	log.Printf("User disconnected from proxy: proxy=%s, user=%s", proxyID, userID)
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
		if err := pm.store.UpdateProxy(conn.toStorageProxy()); err != nil {
			log.Printf("ERROR: Failed to update proxy in database: %v", err)
			return fmt.Errorf("failed to update database: %v", err)
		}
		log.Printf("✅ Updated proxy in database: %s (local: :%d -> %s:%d, protocol: %s)",
			id, localPort, remoteHost, remotePort, protocol)
	} else {
		log.Printf("⚠️  No database store available, proxy updated in memory only")
	}

	log.Printf("Updated proxy connection: %s (local: :%d -> %s:%d, protocol: %s)",
		id, localPort, remoteHost, remotePort, protocol)

	return nil
}

// monitorIdleConnections periodically checks for idle connections and closes them if needed
func (pm *ProxyManager) monitorIdleConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.mu.RLock()
			var toClose []string
			totalPoolCleaned := 0

			for id, conn := range pm.connections {
				// Clean idle connections in pool
				if conn.connPool != nil {
					cleaned := conn.connPool.CleanIdle()
					if cleaned > 0 {
						totalPoolCleaned += cleaned
					}
				}

				if conn.MaxIdleTime > 0 {
					conn.mu.RLock()
					idle := time.Since(conn.LastActive)
					userCount := conn.UserCount
					conn.mu.RUnlock()

					if idle > conn.MaxIdleTime && userCount == 0 {
						toClose = append(toClose, id)
						log.Printf("Proxy %s idle for %v (max: %v), scheduling for closure",
							id, idle, conn.MaxIdleTime)
					}
				}
			}
			pm.mu.RUnlock()

			if totalPoolCleaned > 0 {
				log.Printf("Cleaned %d idle pooled connections across all proxies", totalPoolCleaned)
			}

			// Close idle connections
			for _, id := range toClose {
				if err := pm.CloseProxyConnection(id); err != nil {
					log.Printf("Failed to close idle proxy %s: %v", id, err)
				}
			}

		case <-pm.stopMonitor:
			return
		}
	}
}

// Shutdown stops the proxy manager and closes all connections
func (pm *ProxyManager) Shutdown() {
	close(pm.stopMonitor)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for id := range pm.connections {
		pm.CloseProxyConnection(id)
	}
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

// RestoreProxiesForClient restores saved proxies for a client when they reconnect
func (pm *ProxyManager) RestoreProxiesForClient(clientID string) {
	if pm.store == nil {
		return
	}

	// Get proxies for this client from database
	proxies, err := pm.store.GetProxies(clientID)
	if err != nil {
		log.Printf("Error loading proxies for client %s: %v", clientID, err)
		return
	}

	if len(proxies) == 0 {
		return // No proxies to restore
	}

	log.Printf("Restoring %d proxies for client %s...", len(proxies), clientID)

	for _, proxy := range proxies {
		// Check if this proxy already exists in memory (already running)
		pm.mu.RLock()
		alreadyExists := false
		for _, conn := range pm.connections {
			if conn.ClientID == clientID && conn.LocalPort == proxy.LocalPort {
				alreadyExists = true
				log.Printf("  ℹ️  Proxy already running on :%d, skipping restore", proxy.LocalPort)
				break
			}
		}
		pm.mu.RUnlock()

		if alreadyExists {
			continue
		}

		// Try to recreate the proxy with the ORIGINAL ID from database
		conn, err := pm.createProxyConnectionWithID(
			proxy.ID, // Use the original proxy ID
			proxy.ClientID,
			proxy.RemoteHost,
			proxy.RemotePort,
			proxy.LocalPort,
			proxy.Protocol,
		)

		if err != nil {
			log.Printf("  ⚠️  Failed to restore proxy %s: %v", proxy.ID, err)
			continue
		}

		log.Printf("  ✅ Restored proxy: :%d -> %s:%d (protocol: %s)",
			conn.LocalPort, conn.RemoteHost, conn.RemotePort, conn.Protocol)
	}

	// Clean up old/duplicate proxy records with same client_id and local_port but different IDs
	if pm.store != nil {
		pm.store.CleanupDuplicateProxies(clientID)
	}
}

// --- Adapter methods for pkg/proxy interface ---

// toProxyConnectionInfo converts ProxyConnection to ProxyConnectionInfo for API responses
func (conn *ProxyConnection) toProxyConnectionInfo() proxy.ProxyConnectionInfo {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	return proxy.ProxyConnectionInfo{
		ID:          conn.ID,
		ClientID:    conn.ClientID,
		LocalPort:   conn.LocalPort,
		RemoteHost:  conn.RemoteHost,
		RemotePort:  conn.RemotePort,
		Protocol:    conn.Protocol,
		BytesIn:     conn.BytesIn,
		BytesOut:    conn.BytesOut,
		CreatedAt:   conn.CreatedAt.Format(time.RFC3339),
		LastActive:  conn.LastActive.Format(time.RFC3339),
		UserCount:   conn.UserCount,
		MaxIdleTime: int64(conn.MaxIdleTime.Seconds()),
	}
}

// CreateProxyConnectionInfo implements ProxyManagerInterface
func (pm *ProxyManager) CreateProxyConnectionInfo(clientID, remoteHost string, remotePort, localPort int, protocol string) (proxy.ProxyConnectionInfo, error) {
	conn, err := pm.CreateProxyConnection(clientID, remoteHost, remotePort, localPort, protocol)
	if err != nil {
		return proxy.ProxyConnectionInfo{}, err
	}
	return conn.toProxyConnectionInfo(), nil
}

// ListProxyConnectionsInfo implements ProxyManagerInterface
func (pm *ProxyManager) ListProxyConnectionsInfo(clientID string) []proxy.ProxyConnectionInfo {
	conns := pm.ListProxyConnections(clientID)
	result := make([]proxy.ProxyConnectionInfo, len(conns))
	for i, conn := range conns {
		result[i] = conn.toProxyConnectionInfo()
	}
	return result
}

// ListAllProxyConnectionsInfo implements ProxyManagerInterface
func (pm *ProxyManager) ListAllProxyConnectionsInfo() []proxy.ProxyConnectionInfo {
	conns := pm.ListAllProxyConnections()
	result := make([]proxy.ProxyConnectionInfo, len(conns))
	for i, conn := range conns {
		result[i] = conn.toProxyConnectionInfo()
	}
	return result
}

// GetProxyStatsInfo implements ProxyManagerInterface
func (pm *ProxyManager) GetProxyStatsInfo() map[string]interface{} {
	pm.mu.RLock()
	totalConns := len(pm.connections)
	var totalBytesIn, totalBytesOut int64
	var totalUsers int

	for _, conn := range pm.connections {
		conn.mu.RLock()
		totalBytesIn += conn.BytesIn
		totalBytesOut += conn.BytesOut
		totalUsers += conn.UserCount
		conn.mu.RUnlock()
	}
	pm.mu.RUnlock()

	return map[string]interface{}{
		"total_connections":  totalConns,
		"total_bytes_in":     totalBytesIn,
		"total_bytes_out":    totalBytesOut,
		"total_active_users": totalUsers,
	}
}

// HandleClientGet retrieves or deletes a specific client by ID
func (s *Server) HandleClientGet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleClientLookup(w, r)
	case http.MethodDelete:
		s.handleClientDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClientLookup returns metadata for a single client
func (s *Server) handleClientLookup(w http.ResponseWriter, r *http.Request) {
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

	// Return metadata using the interface
	var meta *protocol.ClientMetadata
	if clientMeta := client.Metadata(); clientMeta == nil {
		// Fallback: return minimal metadata if nil
		meta = &protocol.ClientMetadata{
			ID:     client.ID(),
			Status: "unknown",
		}
	} else {
		meta = clientMeta
		// Ensure ID is populated if missing
		if meta.ID == "" {
			meta.ID = client.ID()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// handleClientDelete removes a client record (and any proxies) and disconnects if connected
func (s *Server) handleClientDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Basic session check to ensure the request comes from the web UI
	if s.webHandler != nil && s.webHandler.sessionMgr != nil {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		if session, exists := s.webHandler.sessionMgr.GetSession(cookie.Value); exists {
			s.webHandler.sessionMgr.RefreshSession(session.ID)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}
	}

	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		clientID = r.PathValue("id")
	}

	if clientID == "" {
		var req struct {
			ID       string `json:"id"`
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.ID != "" {
				clientID = req.ID
			} else {
				clientID = req.ClientID
			}
		}
	}

	if clientID == "" {
		http.Error(w, "Missing client ID", http.StatusBadRequest)
		return
	}

	// Disconnect the client if it is currently connected
	disconnected := s.manager.UnregisterClient(clientID) == nil // Returns nil on success

	// Clear any cached results tied to this client to avoid stale data
	s.clearCachedClientData(clientID)

	// Remove persistent data (client record and proxies)
	if s.store != nil {
		if err := s.store.DeleteClient(clientID); err != nil {
			log.Printf("Failed to delete client %s from store: %v", clientID, err)
			http.Error(w, "Failed to delete client", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "deleted",
		"id":           clientID,
		"disconnected": disconnected,
		"persisted":    s.store != nil,
	})
}

// HandleUpdateClientAlias updates the alias for a client
func (s *Server) HandleUpdateClientAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	clientID := req["client_id"]
	alias := req["alias"]

	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}

	// Update in database
	if s.store != nil {
		if err := s.store.UpdateClientAlias(clientID, alias); err != nil {
			http.Error(w, "Failed to update alias", http.StatusInternalServerError)
			return
		}
	}

	// Update in memory using the interface
	client, exists := s.manager.GetClient(clientID)
	if exists && client != nil {
		client.UpdateMetadata(func(m *protocol.ClientMetadata) {
			if m != nil {
				m.Alias = alias
			}
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "alias": alias})
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

	s.ClearProcessListResult(clientID)

	// Send process list request to client
	msg, err := protocol.NewMessage(protocol.MsgTypeListProcesses, nil)
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := s.manager.SendToClient(clientID, msg); err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	} // Wait for response with timeout (max 30 seconds to allow time for client response)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond) // Poll every 10ms for faster detection
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Printf("Process request timeout for client %s", clientID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
			return
		case <-ticker.C:
			result := s.GetProcessListResult(clientID)
			if result != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				// Ensure Processes is not nil
				processes := result.Processes
				if processes == nil {
					processes = []protocol.Process{}
				}

				if err := json.NewEncoder(w).Encode(processes); err != nil {
					log.Printf("Error encoding processes: %v", err)
				}
				s.ClearProcessListResult(clientID)
				return
			}
		}
	}
}

// HandleSystemInfoAPI serves system information for a client
func (s *Server) HandleSystemInfoAPI(w http.ResponseWriter, r *http.Request) {
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

	s.ClearSystemInfoResult(clientID)

	// Send system info request to client
	msg, err := protocol.NewMessage(protocol.MsgTypeGetSystemInfo, nil)
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := s.manager.SendToClient(clientID, msg); err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	// Wait for response with timeout (max 30 seconds to allow time for client response)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond) // Poll every 10ms for faster detection
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Printf("System info request timeout for client %s", clientID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
			return
		case <-ticker.C:
			result := s.GetSystemInfoResult(clientID)
			if result != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				if err := json.NewEncoder(w).Encode(result); err != nil {
					log.Printf("Error encoding system info: %v", err)
				}
				s.ClearSystemInfoResult(clientID)
				return
			}
		}
	}
}
