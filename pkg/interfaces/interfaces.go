package interfaces

import (
	"context"

	"gorat/pkg/protocol"
)

// Storage defines the interface for persistent data storage
type Storage interface {
	SaveClient(client *protocol.ClientMetadata) error
	GetClient(id string) (*protocol.ClientMetadata, error)
	GetAllClients() ([]*protocol.ClientMetadata, error)
	DeleteClient(id string) error
	UpdateClientStatus(id, status string) error
	SaveProxy(proxy interface{}) error
	GetProxy(id string) (interface{}, error)
	GetProxiesByClient(clientID string) ([]interface{}, error)
	DeleteProxy(id string) error
	GetWebUser(username string) (interface{}, error)
	SaveWebUser(user interface{}) error
	UpdateWebUser(user interface{}) error
	Close() error
}

// ClientConnection defines the interface for a connected client
type ClientConnection interface {
	ID() string
	Metadata() *protocol.ClientMetadata
	Send(msg *protocol.Message) error
	Close() error
	IsClosed() bool
}

// ClientRegistry defines the interface for managing client connections
type ClientRegistry interface {
	Register(client ClientConnection) error
	Unregister(clientID string) error
	Get(clientID string) (ClientConnection, error)
	GetAll() []ClientConnection
	Broadcast(msg *protocol.Message) error
	SendToClient(clientID string, msg *protocol.Message) error
	Run(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ProxyManager defines the interface for managing proxy tunnels
type ProxyManager interface {
	CreateProxy(clientID, remoteHost string, remotePort int) (int, error)
	CloseProxy(proxyID string) error
	GetProxy(proxyID string) (interface{}, error)
	GetProxiesByClient(clientID string) ([]interface{}, error)
	ListAll() ([]interface{}, error)
}

// SessionManager defines the interface for user session management
type SessionManager interface {
	CreateSession(userID, username string) (string, error)
	ValidateSession(sessionID string) (bool, string, error)
	RevokeSession(sessionID string) error
	GetUserFromSession(sessionID string) (string, error)
}

// TerminalProxy defines the interface for remote terminal sessions
type TerminalProxy interface {
	StartSession(clientID, sessionID string) (string, error)
	EndSession(terminalSessionID string) error
	SendInput(terminalSessionID string, input []byte) error
	GetOutput(terminalSessionID string) ([]byte, error)
}

// MessageHandler defines the interface for handling client messages
type MessageHandler interface {
	Handle(ctx context.Context, clientID string, msg *protocol.Message) error
}

// Authenticator defines the interface for client authentication
type Authenticator interface {
	Authenticate(payload *protocol.AuthPayload) (bool, error)
	ValidateToken(token string) (bool, error)
}

// Logger defines the interface for structured logging
type Logger interface {
	DebugWith(msg string, args ...any)
	InfoWith(msg string, args ...any)
	WarnWith(msg string, args ...any)
	ErrorWith(msg string, args ...any)
	ErrorWithErr(msg string, err error, args ...any)
	With(args ...any) Logger
}
