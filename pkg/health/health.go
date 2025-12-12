package health

import (
	"runtime"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	Name        string      `json:"name"`
	Status      Status      `json:"status"`
	Description string      `json:"description,omitempty"`
	LastChecked time.Time   `json:"last_checked"`
	Details     interface{} `json:"details,omitempty"`
}

// ServerHealth represents overall server health
type ServerHealth struct {
	Status         Status            `json:"status"`
	Uptime         int64             `json:"uptime_seconds"`
	Timestamp      time.Time         `json:"timestamp"`
	ActiveClients  int               `json:"active_clients"`
	Goroutines     int               `json:"goroutines"`
	MemoryMB       uint64            `json:"memory_mb"`
	Components     []ComponentHealth `json:"components"`
	ResponseTimeMs int64             `json:"response_time_ms"`
}

// Monitor tracks server health metrics
type Monitor struct {
	startTime  time.Time
	clientsMu  sync.RWMutex
	components map[string]*ComponentHealth
}

// NewMonitor creates a new health monitor
func NewMonitor() *Monitor {
	return &Monitor{
		startTime:  time.Now(),
		components: make(map[string]*ComponentHealth),
	}
}

// SetComponentStatus updates the status of a component
func (m *Monitor) SetComponentStatus(name string, status Status, description string) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	m.components[name] = &ComponentHealth{
		Name:        name,
		Status:      status,
		Description: description,
		LastChecked: time.Now(),
	}
}

// SetComponentStatusWithDetails updates component status with additional details
func (m *Monitor) SetComponentStatusWithDetails(name string, status Status, description string, details interface{}) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	m.components[name] = &ComponentHealth{
		Name:        name,
		Status:      status,
		Description: description,
		LastChecked: time.Now(),
		Details:     details,
	}
}

// GetHealth returns the current server health
func (m *Monitor) GetHealth(activeClients int) *ServerHealth {
	m.clientsMu.RLock()
	components := make([]ComponentHealth, 0, len(m.components))
	overallStatus := StatusHealthy
	for _, comp := range m.components {
		components = append(components, *comp)
		if comp.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if comp.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}
	m.clientsMu.RUnlock()

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	return &ServerHealth{
		Status:        overallStatus,
		Uptime:        int64(time.Since(m.startTime).Seconds()),
		Timestamp:     time.Now(),
		ActiveClients: activeClients,
		Goroutines:    runtime.NumGoroutine(),
		MemoryMB:      stats.Alloc / 1024 / 1024,
		Components:    components,
	}
}
