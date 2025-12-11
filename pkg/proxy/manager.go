package proxy

import (
	"net"
)

// Manager manages proxy tunnel lifecycle and creation
type Manager interface {
	// CreateProxy creates a new proxy tunnel
	CreateProxy(clientID, remoteHost string, remotePort int, protocol string) (proxyID string, err error)

	// CloseProxy closes a proxy tunnel
	CloseProxy(proxyID string) error

	// GetProxy retrieves proxy information
	GetProxy(proxyID string) (info map[string]interface{}, err error)

	// GetProxiesByClient gets all proxies for a client
	GetProxiesByClient(clientID string) (proxies []map[string]interface{}, err error)

	// ListAll returns all active proxies
	ListAll() (proxies []map[string]interface{}, err error)

	// GetClientForProxy returns the client ID for a proxy
	GetClientForProxy(proxyID string) (clientID string, err error)
}

// Pool manages pooled connections to remote hosts
type Pool interface {
	// Get retrieves or creates a connection
	Get() (net.Conn, error)

	// Put returns a connection to the pool
	Put(conn net.Conn) error

	// Close closes all pooled connections
	Close() error

	// Stats returns pool statistics
	Stats() PoolStats
}

// PoolStats contains connection pool statistics
type PoolStats struct {
	TotalConnections int
	AvailableConns   int
	InUseConns       int
	CreatedAt        string
	LastUsed         string
}

// Tunnel represents a single proxy tunnel
type Tunnel interface {
	// ID returns the tunnel identifier
	ID() string

	// Forward starts forwarding data through the tunnel
	Forward() error

	// Close closes the tunnel
	Close() error

	// GetStats returns tunnel statistics
	GetStats() TunnelStats
}

// TunnelStats contains tunnel statistics
type TunnelStats struct {
	ID            string
	ClientID      string
	RemoteHost    string
	RemotePort    int
	BytesSent     int64
	BytesReceived int64
	CreatedAt     string
	LastActivity  string
}
