package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorat/pkg/clients"
	"gorat/pkg/protocol"
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

// HandleDeleteClient deletes a client
func (ah *AdminHandler) HandleDeleteClient(c *gin.Context) {
	clientID := c.Param("id")

	// Disconnect client if connected
	_ = ah.clientMgr.UnregisterClient(clientID)

	// Delete from database
	if ah.store != nil {
		if err := ah.store.DeleteClient(clientID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Client deleted successfully"})
}

// HandleDeleteProxy deletes a proxy tunnel
func (ah *AdminHandler) HandleDeleteProxy(c *gin.Context) {
	proxyID := c.Param("id")

	// Delete from database
	if err := ah.store.DeleteProxy(proxyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Proxy deleted successfully"})
}

// HandleGetSettings retrieves all server settings
func (ah *AdminHandler) HandleGetSettings(c *gin.Context) {
	settings, err := ah.store.GetAllServerSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// HandleSaveSettings saves server settings
func (ah *AdminHandler) HandleSaveSettings(c *gin.Context) {
	var settings map[string]string
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	for key, value := range settings {
		if err := ah.store.SetServerSetting(key, value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings saved successfully"})
}

// HandlePushUpdate sends update commands to clients by platform
func (ah *AdminHandler) HandlePushUpdate(c *gin.Context) {
	var req struct {
		Platform string `json:"platform"`
		Version  string `json:"version"`
		Force    bool   `json:"force"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if req.Platform == "" || req.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Platform and version are required"})
		return
	}

	// Get all connected clients from the manager
	allClients := ah.clientMgr.GetAllClients()

	// Filter clients by platform
	var matchingClients []clients.Client
	if req.Platform != "all" {
		for _, client := range allClients {
			// Convert OS and Arch to platform key
			meta := client.Metadata()
			if meta != nil {
				platform := getPlatformKey(meta.OS, meta.Arch)
				if platform == req.Platform {
					matchingClients = append(matchingClients, client)
				}
			}
		}
	} else {
		matchingClients = allClients
	}

	// Send update command to each matching client
	totalMatching := len(matchingClients)
	updatesSent := 0
	updatesFailed := 0
	var logs []map[string]interface{}

	for _, client := range matchingClients {
		// Build update URL
		updateURL := buildUpdateURL(req.Platform, req.Version, ah.store)
		if updateURL == "" {
			updatesFailed++
			logs = append(logs, map[string]interface{}{
				"timestamp": time.Now().String(),
				"status":    "failed",
				"client_id": client.ID(),
				"message":   "Update URL not configured",
			})
			continue
		}

		// Create update command message
		updatePayload := map[string]interface{}{
			"command": "update",
			"url":     updateURL,
			"version": req.Version,
			"force":   req.Force,
		}

		payloadBytes, _ := json.Marshal(updatePayload)
		msg, err := protocol.NewMessage(protocol.MsgTypeUpdateStatus, payloadBytes)
		if err != nil {
			updatesFailed++
			logs = append(logs, map[string]interface{}{
				"timestamp": time.Now().String(),
				"status":    "failed",
				"client_id": client.ID(),
				"message":   "Failed to create message: " + err.Error(),
			})
			continue
		}

		// Send message to client
		if err := ah.clientMgr.SendToClient(client.ID(), msg); err != nil {
			updatesFailed++
			logs = append(logs, map[string]interface{}{
				"timestamp": time.Now().String(),
				"status":    "failed",
				"client_id": client.ID(),
				"message":   "Failed to send command: " + err.Error(),
			})
		} else {
			updatesSent++
			meta := client.Metadata()
			hostname := ""
			if meta != nil {
				hostname = meta.Hostname
			}
			logs = append(logs, map[string]interface{}{
				"timestamp": time.Now().String(),
				"status":    "success",
				"client_id": client.ID(),
				"message":   "Update command sent to " + hostname,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_matching": totalMatching,
		"updates_sent":   updatesSent,
		"updates_failed": updatesFailed,
		"log":            logs,
	})
}

// getPlatformKey converts OS and Arch to platform key
func getPlatformKey(os, arch string) string {
	osMap := map[string]string{
		"windows": "windows",
		"linux":   "linux",
		"darwin":  "darwin",
	}

	archMap := map[string]string{
		"amd64": "amd64",
		"386":   "386",
		"arm64": "arm64",
	}

	osKey := osMap[os]
	archKey := archMap[arch]

	if osKey == "" || archKey == "" {
		return ""
	}

	return osKey + "-" + archKey
}

// buildUpdateURL constructs the update URL from settings
func buildUpdateURL(platform, version string, store storage.Store) string {
	settingKey := "update_path_" + platform
	basePath, err := store.GetServerSetting(settingKey)
	if err != nil || basePath == "" {
		return ""
	}

	// Replace {version} placeholder
	url := strings.ReplaceAll(basePath, "{version}", version)
	return url
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
