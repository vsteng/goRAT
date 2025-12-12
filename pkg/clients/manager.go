package clients

import (
	"fmt"
	"gorat/pkg/protocol"
	"sync"

	"github.com/gorilla/websocket"
)

// ClientImpl represents a connected client
type ClientImpl struct {
	id       string
	conn     *websocket.Conn
	metadata *protocol.ClientMetadata
	send     chan *protocol.Message
	mu       sync.RWMutex
	closed   bool
	writeMu  sync.Mutex
}

// ID returns the client ID
func (c *ClientImpl) ID() string {
	return c.id
}

// Conn returns the WebSocket connection
func (c *ClientImpl) Conn() *websocket.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// Metadata returns client metadata
func (c *ClientImpl) Metadata() *protocol.ClientMetadata {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metadata
}

// UpdateMetadata updates client metadata
func (c *ClientImpl) UpdateMetadata(fn func(*protocol.ClientMetadata)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metadata != nil && !c.closed {
		fn(c.metadata)
	}
}

// SendMessage sends a message to the client
func (c *ClientImpl) SendMessage(msg *protocol.Message) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client %s is closed", c.id)
	}
	send := c.send
	c.mu.RUnlock()

	select {
	case send <- msg:
		return nil
	default:
		return fmt.Errorf("send buffer full for client %s", c.id)
	}
}

// Close closes the client connection
func (c *ClientImpl) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	c.mu.Unlock()

	close(c.send)
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// IsClosed checks if the client is closed
func (c *ClientImpl) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// ManagerImpl manages all connected clients
type ManagerImpl struct {
	clients    map[string]*ClientImpl
	register   chan *ClientImpl
	unregister chan string
	broadcast  chan *protocol.Message
	mu         sync.RWMutex
	running    bool
	stopOnce   sync.Once
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewManager creates a new client manager
func NewManager() *ManagerImpl {
	return &ManagerImpl{
		clients:    make(map[string]*ClientImpl),
		register:   make(chan *ClientImpl, 256),
		unregister: make(chan string, 256),
		broadcast:  make(chan *protocol.Message, 256),
		stopChan:   make(chan struct{}),
	}
}

// RegisterClient registers a new connected client
func (m *ManagerImpl) RegisterClient(clientID string, conn *websocket.Conn) (Client, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	client := &ClientImpl{
		id:       clientID,
		conn:     conn,
		metadata: &protocol.ClientMetadata{ID: clientID},
		send:     make(chan *protocol.Message, 256),
		closed:   false,
	}

	m.mu.Lock()
	_, exists := m.clients[clientID]
	m.mu.Unlock()

	if exists {
		conn.Close()
		return nil, fmt.Errorf("client %s already connected", clientID)
	}

	select {
	case m.register <- client:
		return client, nil
	case <-m.stopChan:
		conn.Close()
		return nil, fmt.Errorf("manager is stopped")
	}
}

// UnregisterClient removes a client from the manager
func (m *ManagerImpl) UnregisterClient(clientID string) error {
	select {
	case m.unregister <- clientID:
		return nil
	case <-m.stopChan:
		return fmt.Errorf("manager is stopped")
	}
}

// GetClient retrieves a client by ID
func (m *ManagerImpl) GetClient(clientID string) (Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[clientID]
	return client, ok
}

// GetAllClients returns all connected clients
func (m *ManagerImpl) GetAllClients() []Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	return clients
}

// UpdateClientMetadata updates metadata for a client
func (m *ManagerImpl) UpdateClientMetadata(clientID string, fn func(*protocol.ClientMetadata)) error {
	m.mu.RLock()
	client, ok := m.clients[clientID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}

	client.UpdateMetadata(fn)
	return nil
}

// BroadcastMessage sends a message to all connected clients
func (m *ManagerImpl) BroadcastMessage(msg *protocol.Message) {
	select {
	case m.broadcast <- msg:
	case <-m.stopChan:
	}
}

// Start starts the client manager event loop
func (m *ManagerImpl) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.wg.Add(1)
	go m.run()
}

// Stop gracefully stops the client manager
func (m *ManagerImpl) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	// Use sync.Once to ensure stop channel is only closed once
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
	m.wg.Wait()

	m.mu.Lock()
	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]*ClientImpl)
	m.mu.Unlock()
}

// IsRunning checks if the manager is running
func (m *ManagerImpl) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// run is the main event loop for the manager
func (m *ManagerImpl) run() {
	defer m.wg.Done()

	for {
		select {
		case client := <-m.register:
			m.handleRegister(client)

		case clientID := <-m.unregister:
			m.handleUnregister(clientID)

		case msg := <-m.broadcast:
			m.handleBroadcast(msg)

		case <-m.stopChan:
			return
		}
	}
}

// handleRegister handles client registration
func (m *ManagerImpl) handleRegister(client *ClientImpl) {
	m.mu.Lock()
	m.clients[client.id] = client
	m.mu.Unlock()

	m.wg.Add(1)
	go m.handleClientMessages(client)
}

// handleUnregister handles client unregistration
func (m *ManagerImpl) handleUnregister(clientID string) {
	m.mu.Lock()
	client, ok := m.clients[clientID]
	if ok {
		delete(m.clients, clientID)
	}
	m.mu.Unlock()

	if ok {
		client.Close()
	}
}

// handleBroadcast broadcasts a message to all connected clients
func (m *ManagerImpl) handleBroadcast(msg *protocol.Message) {
	m.mu.RLock()
	clients := make([]*ClientImpl, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	m.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- msg:
		default:
			// Skip client with full buffer
		}
	}
}

// handleClientMessages handles outgoing messages for a client
func (m *ManagerImpl) handleClientMessages(client *ClientImpl) {
	defer m.wg.Done()

	for msg := range client.send {
		client.writeMu.Lock()
		err := client.conn.WriteJSON(msg)
		client.writeMu.Unlock()

		if err != nil {
			m.UnregisterClient(client.id)
			break
		}
	}
}

// SendToClient sends a message to a specific client
func (m *ManagerImpl) SendToClient(clientID string, msg *protocol.Message) error {
	client, ok := m.GetClient(clientID)
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}
	return client.SendMessage(msg)
}

// GetClientCount returns the number of connected clients
func (m *ManagerImpl) GetClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// IsClientIDRegistered checks if a client ID is already registered
func (m *ManagerImpl) IsClientIDRegistered(clientID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.clients[clientID]
	return exists
}
