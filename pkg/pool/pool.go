package pool

import (
	"net"
	"sync"
	"time"
)

// Default configuration values
const (
	DefaultMaxPooledConns   = 10               // Maximum connections per remote host
	DefaultPoolConnIdleTime = 5 * time.Minute  // Idle timeout
	DefaultPoolConnLifetime = 30 * time.Minute // Max connection lifetime
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
		connections: make([]*PooledConnection, 0, DefaultMaxPooledConns),
		maxConns:    DefaultMaxPooledConns,
		idleTimeout: DefaultPoolConnIdleTime,
		maxLifetime: DefaultPoolConnLifetime,
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
