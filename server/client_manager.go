package server

import (
	"sync"
	"time"

	"gorat/pkg/logger"
	"gorat/pkg/protocol"
	"gorat/pkg/storage"

	"github.com/gorilla/websocket"
)

// Client represents a connected client
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Metadata *protocol.ClientMetadata
	Send     chan *protocol.Message
	mu       sync.RWMutex
	closed   bool       // Track if Send channel is closed
	writeMu  sync.Mutex // Protects WebSocket writes (not thread-safe)
}

// ClientManager manages all connected clients
type ClientManager struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *protocol.Message
	store      storage.Store // Reference to persistent storage
	mu         sync.RWMutex
	running    bool
	runningMu  sync.Mutex
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *protocol.Message, 256),
	}
}

// SetStore sets the client store reference
func (m *ClientManager) SetStore(store storage.Store) {
	m.store = store
}

// safeCloseClient safely closes a client's Send channel
func (m *ClientManager) safeCloseClient(client *Client) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if !client.closed {
		close(client.Send)
		client.closed = true
	}
}

// Run starts the client manager
func (m *ClientManager) Run() {
	// Prevent multiple instances
	m.runningMu.Lock()
	if m.running {
		logger.Get().Warn("ClientManager.Run() already running, skipping duplicate start")
		m.runningMu.Unlock()
		return
	}
	m.running = true
	m.runningMu.Unlock()

	defer func() {
		m.runningMu.Lock()
		m.running = false
		m.runningMu.Unlock()

		if r := recover(); r != nil {
			logger.Get().ErrorWith("panic recovered in ClientManager.Run", "panic", r)
			logger.Get().Info("recreating channels and restarting client manager in 2 seconds")

			// Recreate channels to avoid issues with closed channels
			m.mu.Lock()
			m.register = make(chan *Client)
			m.unregister = make(chan *Client)
			m.broadcast = make(chan *protocol.Message, 256)
			m.mu.Unlock()

			time.Sleep(2 * time.Second)
			go m.Run() // Restart the manager
		}
	}()

	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			// Check if client ID already exists
			if existing, exists := m.clients[client.ID]; exists {
				// Client ID already registered - close the existing connection
				logger.Get().InfoWith("client ID already exists, closing old connection", "clientID", client.ID)
				m.safeCloseClient(existing)
				existing.Conn.Close()
			}
			m.clients[client.ID] = client
			m.mu.Unlock()
			logger.Get().InfoWith("client registered", "clientID", client.ID, "hostname", client.Metadata.Hostname)

		case client := <-m.unregister:
			m.mu.Lock()
			// Only unregister if this is the currently registered client
			// (not an old connection that was already replaced)
			if current, ok := m.clients[client.ID]; ok && current == client {
				m.safeCloseClient(client)
				delete(m.clients, client.ID)
				logger.Get().InfoWith("client unregistered", "clientID", client.ID)
			} else if ok {
				// This is an old connection being cleaned up, ignore
				logger.Get().DebugWith("ignoring unregister for old connection", "clientID", client.ID)
			}
			m.mu.Unlock()

		case message := <-m.broadcast:
			m.mu.Lock() // Use Lock instead of RLock since we may delete
			for id, client := range m.clients {
				select {
				case client.Send <- message:
				default:
					// Channel full or closed, remove client
					m.safeCloseClient(client)
					delete(m.clients, id)
				}
			}
			m.mu.Unlock()
		}
	}
}

// GetClient returns a client by ID
func (m *ClientManager) GetClient(id string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[id]
	return client, ok
}

// RemoveClient forcibly disconnects and forgets a client by ID
func (m *ClientManager) RemoveClient(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[id]
	if !ok {
		return false
	}

	m.safeCloseClient(client)
	if client.Conn != nil {
		client.Conn.Close()
	}

	delete(m.clients, id)
	return true
}

// GetAllClients returns all connected clients
func (m *ClientManager) GetAllClients() []*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clients := make([]*Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	return clients
}

// SendToClient sends a message to a specific client
func (m *ClientManager) SendToClient(clientID string, msg *protocol.Message) error {
	client, ok := m.GetClient(clientID)
	if !ok {
		return ErrClientNotFound
	}

	select {
	case client.Send <- msg:
		return nil
	case <-time.After(5 * time.Second):
		return ErrSendTimeout
	}
}

// Broadcast sends a message to all clients
func (m *ClientManager) Broadcast(msg *protocol.Message) {
	m.broadcast <- msg
}

// GetClientCount returns the number of connected clients
func (m *ClientManager) GetClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// UpdateClientMetadata updates client metadata
func (m *ClientManager) UpdateClientMetadata(clientID string, update func(*protocol.ClientMetadata)) error {
	client, ok := m.GetClient(clientID)
	if !ok {
		return ErrClientNotFound
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	update(client.Metadata)
	return nil
}

// IsClientIDRegistered checks if a client ID is already registered
func (m *ClientManager) IsClientIDRegistered(clientID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.clients[clientID]
	return exists
}

// GetClients returns metadata for all connected clients
func (m *ClientManager) GetClients() []*protocol.ClientMetadata {
	m.mu.RLock()
	connectedClients := make(map[string]*protocol.ClientMetadata)
	for _, client := range m.clients {
		client.mu.RLock()
		connectedClients[client.ID] = client.Metadata
		client.mu.RUnlock()
	}
	m.mu.RUnlock()

	// If we have a store, merge with saved clients
	if m.store != nil {
		savedClients, err := m.store.GetAllClients()
		if err == nil {
			// Create a map to merge clients
			allClients := make(map[string]*protocol.ClientMetadata)

			// First add all saved clients
			for _, saved := range savedClients {
				allClients[saved.ID] = saved
			}

			// Override with connected clients (they have latest data)
			for id, connected := range connectedClients {
				allClients[id] = connected
			}

			// Convert to slice and sort by last_seen (most recent first)
			metadata := make([]*protocol.ClientMetadata, 0, len(allClients))
			for _, client := range allClients {
				metadata = append(metadata, client)
			}

			// Sort by last_seen descending
			sortClientsByLastSeen(metadata)

			return metadata
		}
	}

	// Fallback: return only connected clients
	metadata := make([]*protocol.ClientMetadata, 0, len(connectedClients))
	for _, client := range connectedClients {
		metadata = append(metadata, client)
	}

	// Sort by last_seen descending
	sortClientsByLastSeen(metadata)

	return metadata
}

// sortClientsByLastSeen sorts clients by last_seen in descending order
func sortClientsByLastSeen(clients []*protocol.ClientMetadata) {
	// Simple bubble sort for small datasets
	for i := 0; i < len(clients); i++ {
		for j := i + 1; j < len(clients); j++ {
			if clients[i].LastSeen.Before(clients[j].LastSeen) {
				clients[i], clients[j] = clients[j], clients[i]
			}
		}
	}
}
