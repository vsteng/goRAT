package clients

import (
	"gorat/pkg/protocol"

	"github.com/gorilla/websocket"
)

// Client represents a connected client with metadata and messaging capability
type Client interface {
	// ID returns the client ID
	ID() string
	// Conn returns the WebSocket connection
	Conn() *websocket.Conn
	// Metadata returns client metadata
	Metadata() *protocol.ClientMetadata
	// UpdateMetadata updates client metadata
	UpdateMetadata(fn func(*protocol.ClientMetadata))
	// SendMessage sends a message to the client
	SendMessage(msg *protocol.Message) error
	// SendRaw sends a raw JSON payload using the client's write lock (for non-protocol messages)
	SendRaw(fn func(conn *websocket.Conn) error) error
	// Close closes the client connection
	Close() error
	// IsClosed checks if the client is closed
	IsClosed() bool
}

// Manager manages all connected clients and their lifecycle
type Manager interface {
	// RegisterClient registers a new connected client
	RegisterClient(clientID string, conn *websocket.Conn) (Client, error)
	// UnregisterClient removes a client from the manager
	UnregisterClient(clientID string) error
	// GetClient retrieves a client by ID
	GetClient(clientID string) (Client, bool)
	// GetAllClients returns all connected clients
	GetAllClients() []Client
	// UpdateClientMetadata updates metadata for a client
	UpdateClientMetadata(clientID string, fn func(*protocol.ClientMetadata)) error
	// BroadcastMessage sends a message to all connected clients
	BroadcastMessage(msg *protocol.Message)
	// SendToClient sends a message to a specific client
	SendToClient(clientID string, msg *protocol.Message) error
	// GetClientCount returns the number of connected clients
	GetClientCount() int
	// IsClientIDRegistered checks if a client ID is already registered
	IsClientIDRegistered(clientID string) bool
	// Start starts the client manager event loop
	Start()
	// Stop gracefully stops the client manager
	Stop()
	// IsRunning checks if the manager is running
	IsRunning() bool
}
