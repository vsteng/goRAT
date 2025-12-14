package proxy

import (
	"net/http"
	"strconv"

	"gorat/pkg/clients"
	"gorat/pkg/logger"
	"gorat/pkg/storage"

	"github.com/gin-gonic/gin"
)

// ProxyHandler handles proxy-related HTTP API requests
type ProxyHandler struct {
	manager      clients.Manager
	store        storage.Store
	proxyManager ProxyManagerInterface
}

// ProxyManagerInterface defines the interface for proxy management operations
type ProxyManagerInterface interface {
	CreateProxyConnectionInfo(clientID, remoteHost string, remotePort, localPort int, protocol string) (ProxyConnectionInfo, error)
	ListProxyConnectionsInfo(clientID string) []ProxyConnectionInfo
	ListAllProxyConnectionsInfo() []ProxyConnectionInfo
	CloseProxyConnection(id string) error
	GetSuggestedPorts(basePort int, count int) []int
	UpdateProxyConnection(id, remoteHost string, remotePort, localPort int, protocol string) error
	GetProxyStatsInfo() map[string]interface{}
}

// ProxyConnectionInfo represents proxy connection information for API responses
type ProxyConnectionInfo struct {
	ID          string `json:"ID"`
	ClientID    string `json:"ClientID"`
	LocalPort   int    `json:"LocalPort"`
	RemoteHost  string `json:"RemoteHost"`
	RemotePort  int    `json:"RemotePort"`
	Protocol    string `json:"Protocol"`
	BytesIn     int64  `json:"BytesIn"`
	BytesOut    int64  `json:"BytesOut"`
	CreatedAt   string `json:"CreatedAt"`
	LastActive  string `json:"LastActive"`
	UserCount   int    `json:"UserCount"`
	MaxIdleTime int64  `json:"MaxIdleTime"`
	Status      string `json:"Status"`
}

// NewProxyHandler creates a new ProxyHandler
func NewProxyHandler(manager clients.Manager, store storage.Store, proxyManager ProxyManagerInterface) *ProxyHandler {
	return &ProxyHandler{
		manager:      manager,
		store:        store,
		proxyManager: proxyManager,
	}
}

// HandleProxyCreate handles creating a new proxy connection
func (h *ProxyHandler) HandleProxyCreate(c *gin.Context) {
	var rawReq map[string]interface{}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Extract fields, supporting both snake_case and camelCase
	clientID := extractString(rawReq, "client_id", "clientId")
	remoteHost := extractString(rawReq, "remote_host", "remoteHost")
	remotePort := extractInt(rawReq, "remote_port", "remotePort")
	localPort := extractInt(rawReq, "local_port", "localPort")
	protocol := extractString(rawReq, "protocol", "protocol")

	// Validate required fields
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing client_id"})
		return
	}
	if remoteHost == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing remote_host"})
		return
	}
	if remotePort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing remote_port"})
		return
	}
	if localPort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing local_port"})
		return
	}

	if protocol == "" {
		protocol = "tcp"
	}

	conn, err := h.proxyManager.CreateProxyConnectionInfo(clientID, remoteHost, remotePort, localPort, protocol)
	if err != nil {
		logger.Get().ErrorWithErr("failed to create proxy connection", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Get().InfoWith("proxy connection created",
		"proxyID", conn.ID,
		"clientID", clientID,
		"localPort", localPort,
		"remoteHost", remoteHost,
		"remotePort", remotePort)

	c.JSON(http.StatusOK, conn)
}

// HandleProxyList lists proxy connections
func (h *ProxyHandler) HandleProxyList(c *gin.Context) {
	// Support both snake_case and camelCase
	clientID := c.Query("clientId")
	if clientID == "" {
		clientID = c.Query("client_id")
	}

	var connections []ProxyConnectionInfo
	if clientID != "" {
		connections = h.proxyManager.ListProxyConnectionsInfo(clientID)
	} else {
		connections = h.proxyManager.ListAllProxyConnectionsInfo()
	}

	c.JSON(http.StatusOK, connections)
}

// HandleProxyClose closes a proxy connection
func (h *ProxyHandler) HandleProxyClose(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing proxy ID"})
		return
	}

	if err := h.proxyManager.CloseProxyConnection(id); err != nil {
		logger.Get().ErrorWithErr("failed to close proxy connection", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	logger.Get().InfoWith("proxy connection closed", "proxyID", id)
	c.JSON(http.StatusOK, gin.H{"status": "closed"})
}

// HandleProxySuggestPorts suggests available ports for a new proxy
func (h *ProxyHandler) HandleProxySuggestPorts(c *gin.Context) {
	basePort := 10000 // Default base port
	if bp := c.Query("basePort"); bp != "" {
		if p, err := strconv.Atoi(bp); err == nil {
			basePort = p
		}
	}

	count := 5 // Default number of suggestions
	if countStr := c.Query("count"); countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 && n <= 20 {
			count = n
		}
	}

	suggested := h.proxyManager.GetSuggestedPorts(basePort, count)

	c.JSON(http.StatusOK, gin.H{
		"basePort":       basePort,
		"suggestedPorts": suggested,
	})
}

// HandleProxyEdit updates an existing proxy connection
func (h *ProxyHandler) HandleProxyEdit(c *gin.Context) {
	var rawReq map[string]interface{}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Extract fields
	proxyID := extractString(rawReq, "proxy_id", "proxyId")
	remoteHost := extractString(rawReq, "remote_host", "remoteHost")
	remotePort := extractInt(rawReq, "remote_port", "remotePort")
	localPort := extractInt(rawReq, "local_port", "localPort")
	protocol := extractString(rawReq, "protocol", "protocol")

	// Validate required fields
	if proxyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing proxy_id"})
		return
	}
	if remoteHost == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing remote_host"})
		return
	}
	if remotePort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing remote_port"})
		return
	}
	if localPort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing local_port"})
		return
	}

	if protocol == "" {
		protocol = "tcp"
	}

	if err := h.proxyManager.UpdateProxyConnection(proxyID, remoteHost, remotePort, localPort, protocol); err != nil {
		logger.Get().ErrorWithErr("failed to update proxy connection", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Get().InfoWith("proxy connection updated",
		"proxyID", proxyID,
		"localPort", localPort,
		"remoteHost", remoteHost,
		"remotePort", remotePort)

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// HandleProxyStats returns proxy statistics
func (h *ProxyHandler) HandleProxyStats(c *gin.Context) {
	stats := h.proxyManager.GetProxyStatsInfo()
	c.JSON(http.StatusOK, stats)
}

// HandleProxyFileServer serves files through a proxy connection
func (h *ProxyHandler) HandleProxyFileServer(c *gin.Context) {
	// This method handles file proxying through client connections
	// Implementation would be similar to the original ProxyFileServer
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{"error": "File proxy not yet implemented in new handler"})
}

// Helper functions to extract values from map with fallback keys
func extractString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
}

func extractInt(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if v, ok := m[key].(float64); ok {
			return int(v)
		}
	}
	return 0
}
