package storage

import (
	"time"

	"gorat/pkg/protocol"
)

// Store defines the interface for persistent storage operations
type Store interface {
	// Client operations
	SaveClient(metadata *protocol.ClientMetadata) error
	GetClient(id string) (*protocol.ClientMetadata, error)
	GetAllClients() ([]*protocol.ClientMetadata, error)
	MarkOffline(timeout time.Duration) error
	DeleteClient(id string) error
	UpdateClientAlias(clientID, alias string) error
	GetStats() (total, online, offline int, err error)

	// Proxy operations
	SaveProxy(proxy *ProxyConnection) error
	GetProxies(clientID string) ([]*ProxyConnection, error)
	GetAllProxies() ([]*ProxyConnection, error)
	DeleteProxy(id string) error
	UpdateProxy(proxy *ProxyConnection) error
	CleanupDuplicateProxies(clientID string) error

	// Web user operations
	CreateWebUser(username, passwordHash, fullName, role string) error
	GetWebUser(username string) (*WebUser, string, error)
	UpdateWebUserLastLogin(username string) error
	GetAllWebUsers() ([]*WebUser, error)
	DeleteWebUser(username string) error
	UserExists(username string) (bool, error)
	AdminExists() (bool, error)
	UpdateWebUser(username string, fullName, passwordHash *string) error // partial update helper

	// Server settings operations
	GetServerSetting(key string) (string, error)
	SetServerSetting(key, value string) error
	GetAllServerSettings() (map[string]string, error)
	DeleteServerSetting(key string) error

	// Lifecycle
	Close() error
}

// ProxyConnection represents a proxy tunnel connection
type ProxyConnection struct {
	ID          string
	ClientID    string
	LocalPort   int
	RemoteHost  string
	RemotePort  int
	Protocol    string // "tcp", "http", "https"
	BytesIn     int64
	BytesOut    int64
	CreatedAt   time.Time
	LastActive  time.Time
	UserCount   int
	MaxIdleTime time.Duration
}

// WebUser represents a web UI user
type WebUser struct {
	ID        int
	Username  string
	FullName  string
	Role      string // "admin" or "user"
	Status    string // "active" or "inactive"
	CreatedAt time.Time
	LastLogin *time.Time
}
