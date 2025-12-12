package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorat/pkg/protocol"
	"gorat/pkg/clients"
	"gorat/pkg/storage"

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
	_ = s.manager.UnregisterClient(clientID)

	// Delete from database
	if s.store != nil {
		if err := s.store.DeleteClient(clientID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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

// AdminGetSettingsHandler retrieves all server settings
func (s *Server) AdminGetSettingsHandler(c *gin.Context) {
	settings, err := s.store.GetAllServerSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// AdminSaveSettingsHandler saves server settings
func (s *Server) AdminSaveSettingsHandler(c *gin.Context) {
	var settings map[string]string
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	for key, value := range settings {
		if err := s.store.SetServerSetting(key, value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings saved successfully"})
}

// ginHandleGetSettings retrieves all server settings (public API)
func (s *Server) ginHandleGetSettings(c *gin.Context) {
	s.AdminGetSettingsHandler(c)
}

// ginHandleSaveSettings saves server settings (public API)
func (s *Server) ginHandleSaveSettings(c *gin.Context) {
	s.AdminSaveSettingsHandler(c)
}

// ginHandlePushUpdate sends update commands to clients by platform
func (s *Server) ginHandlePushUpdate(c *gin.Context) {
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
	allClients := s.manager.GetAllClients()

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
		updateURL := buildUpdateURL(req.Platform, req.Version, s.store)
		if updateURL == "" {
			updatesFailed++
			logs = append(logs, map[string]interface{}{
				"timestamp": time.Now().String(),
				"status":    "failed",
				"client_id": client.ID,
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
		if err := s.manager.SendToClient(client.ID(), msg); err != nil {
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
