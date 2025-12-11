package api

import (
	"log"
	"net/http"
	"strconv"

	"gorat/pkg/clients"
	"gorat/pkg/storage"

	"github.com/gin-gonic/gin"
)

// AdminHandler encapsulates admin-specific endpoints
type AdminHandler struct {
	clientMgr clients.Manager
	store     storage.Store
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(clientMgr clients.Manager, store storage.Store) *AdminHandler {
	return &AdminHandler{
		clientMgr: clientMgr,
		store:     store,
	}
}

// HandleClientsList returns paginated list of clients
func (ah *AdminHandler) HandleClientsList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20
	offset := (page - 1) * pageSize

	storageClients, err := ah.store.GetAllClients()
	if err != nil {
		GinRespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalClients := len(storageClients)
	totalPages := (totalClients + pageSize - 1) / pageSize

	endIdx := offset + pageSize
	if endIdx > len(storageClients) {
		endIdx = len(storageClients)
	}

	paginatedClients := storageClients
	if offset < len(storageClients) {
		paginatedClients = storageClients[offset:endIdx]
	}

	c.JSON(http.StatusOK, gin.H{
		"clients":    paginatedClients,
		"page":       page,
		"pageSize":   pageSize,
		"total":      totalClients,
		"totalPages": totalPages,
	})
}

// HandleProxyList returns paginated list of proxies
func (ah *AdminHandler) HandleProxyList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20
	offset := (page - 1) * pageSize

	proxies, err := ah.store.GetAllProxies()
	if err != nil {
		GinRespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalProxies := len(proxies)
	totalPages := (totalProxies + pageSize - 1) / pageSize

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

// HandleUsersList returns paginated list of web users
func (ah *AdminHandler) HandleUsersList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := 20
	offset := (page - 1) * pageSize

	users, err := ah.store.GetAllWebUsers()
	if err != nil {
		GinRespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalUsers := len(users)
	totalPages := (totalUsers + pageSize - 1) / pageSize

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

// HandleGetStats returns server statistics
func (ah *AdminHandler) HandleGetStats(c *gin.Context) {
	total, online, offline, err := ah.store.GetStats()
	if err != nil {
		GinRespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   total,
		"online":  online,
		"offline": offline,
	})
}

// HandleKillClient terminates a client connection
func (ah *AdminHandler) HandleKillClient(c *gin.Context) {
	clientID := c.Param("client_id")
	if clientID == "" {
		GinRespondError(c, http.StatusBadRequest, "client_id required")
		return
	}

	if err := ah.clientMgr.UnregisterClient(clientID); err != nil {
		log.Printf("Error unregistering client %s: %v", clientID, err)
		GinRespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	GinRespondSuccess(c, nil, "Client terminated")
}

// RegisterAdminRoutes registers all admin routes with a Gin router
func (ah *AdminHandler) RegisterAdminRoutes(router *gin.Engine) {
	admin := router.Group("/api/admin")

	// Client management
	admin.GET("/clients", ah.HandleClientsList)
	admin.DELETE("/clients/:client_id", ah.HandleKillClient)

	// Proxy management
	admin.GET("/proxies", ah.HandleProxyList)

	// User management
	admin.GET("/users", ah.HandleUsersList)

	// Settings & stats
	admin.GET("/stats", ah.HandleGetStats)
}
