package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AdminClientHandler handles client list and management
func (s *Server) AdminClientHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20

	offset := (page - 1) * pageSize

	// Get total count
	clients, err := s.store.GetAllClients()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalClients := len(clients)
	totalPages := (totalClients + pageSize - 1) / pageSize

	// Paginate
	endIdx := offset + pageSize
	if endIdx > len(clients) {
		endIdx = len(clients)
	}

	paginatedClients := clients
	if offset < len(clients) {
		paginatedClients = clients[offset:endIdx]
	}

	c.JSON(http.StatusOK, gin.H{
		"clients":    paginatedClients,
		"page":       page,
		"pageSize":   pageSize,
		"total":      totalClients,
		"totalPages": totalPages,
	})
}

// AdminProxyHandler handles proxy tunnel list and management
func (s *Server) AdminProxyHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20

	offset := (page - 1) * pageSize

	// Get all proxies
	proxies, err := s.store.GetAllProxies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalProxies := len(proxies)
	totalPages := (totalProxies + pageSize - 1) / pageSize

	// Paginate
	endIdx := offset + pageSize
	if endIdx > len(proxies) {
		endIdx = len(proxies)
	}

	paginatedProxies := proxies
	if offset < len(proxies) {
		paginatedProxies = proxies[offset:endIdx]
	}

	c.JSON(http.StatusOK, gin.H{
		"proxies":    paginatedProxies,
		"page":       page,
		"pageSize":   pageSize,
		"total":      totalProxies,
		"totalPages": totalPages,
	})
}

// AdminUserHandler handles web user list and management
func (s *Server) AdminUserHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20

	offset := (page - 1) * pageSize

	// Get all users
	users, err := s.store.GetAllWebUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalUsers := len(users)
	totalPages := (totalUsers + pageSize - 1) / pageSize

	// Paginate
	endIdx := offset + pageSize
	if endIdx > len(users) {
		endIdx = len(users)
	}

	paginatedUsers := users
	if offset < len(users) {
		paginatedUsers = users[offset:endIdx]
	}

	c.JSON(http.StatusOK, gin.H{
		"users":      paginatedUsers,
		"page":       page,
		"pageSize":   pageSize,
		"total":      totalUsers,
		"totalPages": totalPages,
	})
}

// AdminDeleteClientHandler deletes a client
func (s *Server) AdminDeleteClientHandler(c *gin.Context) {
	clientID := c.Param("id")

	// Disconnect client if connected
	s.manager.RemoveClient(clientID)

	// Delete from database
	if err := s.store.DeleteClient(clientID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Client deleted successfully"})
}

// AdminDeleteProxyHandler deletes a proxy tunnel
func (s *Server) AdminDeleteProxyHandler(c *gin.Context) {
	proxyID := c.Param("id")

	// Close and remove from manager if exists
	_ = s.proxyManager.CloseProxyConnection(proxyID)

	// Delete from database
	if err := s.store.DeleteProxy(proxyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Proxy deleted successfully"})
}

// AdminStatsHandler returns dashboard statistics
func (s *Server) AdminStatsHandler(c *gin.Context) {
	clients, _ := s.store.GetAllClients()
	proxies, _ := s.store.GetAllProxies()
	users, _ := s.store.GetAllWebUsers()

	onlineCount := 0
	for _, client := range clients {
		if _, ok := s.manager.GetClient(client.ID); ok {
			onlineCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"totalClients":  len(clients),
		"onlineClients": onlineCount,
		"totalProxies":  len(proxies),
		"totalUsers":    len(users),
	})
}
