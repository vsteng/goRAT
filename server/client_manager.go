package server

import (
	"log"
	"sync"
	"time"

	"mww2.com/server_manager/common"

	"github.com/gorilla/websocket"
)

// Client represents a connected client
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Metadata *common.ClientMetadata
	Send     chan *common.Message
	mu       sync.RWMutex
}

// ClientManager manages all connected clients
type ClientManager struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *common.Message
	store      *ClientStore // Reference to persistent storage
	mu         sync.RWMutex
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *common.Message, 256),
	}
}

// SetStore sets the client store reference
func (m *ClientManager) SetStore(store *ClientStore) {
	m.store = store
}

// Run starts the client manager
func (m *ClientManager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			// Check if client ID already exists
			if existing, exists := m.clients[client.ID]; exists {
				// Client ID already registered - close the existing connection
				log.Printf("Client ID %s already exists, closing old connection", client.ID)
				close(existing.Send)
				existing.Conn.Close()
			}
			m.clients[client.ID] = client
			m.mu.Unlock()
			log.Printf("Client registered: %s (%s)", client.ID, client.Metadata.Hostname)

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client.ID]; ok {
				close(client.Send)
				delete(m.clients, client.ID)
				log.Printf("Client unregistered: %s", client.ID)
			}
			m.mu.Unlock()

		case message := <-m.broadcast:
			m.mu.RLock()
			for _, client := range m.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(m.clients, client.ID)
				}
			}
			m.mu.RUnlock()
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
func (m *ClientManager) SendToClient(clientID string, msg *common.Message) error {
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
func (m *ClientManager) Broadcast(msg *common.Message) {
	m.broadcast <- msg
}

// GetClientCount returns the number of connected clients
func (m *ClientManager) GetClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// UpdateClientMetadata updates client metadata
func (m *ClientManager) UpdateClientMetadata(clientID string, update func(*common.ClientMetadata)) error {
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
func (m *ClientManager) GetClients() []*common.ClientMetadata {
	m.mu.RLock()
	connectedClients := make(map[string]*common.ClientMetadata)
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
			allClients := make(map[string]*common.ClientMetadata)

			// First add all saved clients
			for _, saved := range savedClients {
				allClients[saved.ID] = saved
			}

			// Override with connected clients (they have latest data)
			for id, connected := range connectedClients {
				allClients[id] = connected
			}

			// Convert to slice and sort by last_seen (most recent first)
			metadata := make([]*common.ClientMetadata, 0, len(allClients))
			for _, client := range allClients {
				metadata = append(metadata, client)
			}

			// Sort by last_seen descending
			sortClientsByLastSeen(metadata)

			return metadata
		}
	}

	// Fallback: return only connected clients
	metadata := make([]*common.ClientMetadata, 0, len(connectedClients))
	for _, client := range connectedClients {
		metadata = append(metadata, client)
	}

	// Sort by last_seen descending
	sortClientsByLastSeen(metadata)

	return metadata
}

// sortClientsByLastSeen sorts clients by last_seen in descending order
func sortClientsByLastSeen(clients []*common.ClientMetadata) {
	// Simple bubble sort for small datasets
	for i := 0; i < len(clients); i++ {
		for j := i + 1; j < len(clients); j++ {
			if clients[i].LastSeen.Before(clients[j].LastSeen) {
				clients[i], clients[j] = clients[j], clients[i]
			}
		}
	}
}
