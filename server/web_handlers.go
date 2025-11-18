package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"mww2.com/server_manager/common"
)

// WebConfig holds web UI configuration
type WebConfig struct {
	Username string
	Password string
}

// WebHandler handles web UI requests
type WebHandler struct {
	sessionMgr *SessionManager
	clientMgr  *ClientManager
	config     *WebConfig
	templates  *template.Template
	server     *Server // Reference to main server for result access
}

// NewWebHandler creates a new web handler
func NewWebHandler(sessionMgr *SessionManager, clientMgr *ClientManager, config *WebConfig) (*WebHandler, error) {
	// Load templates from disk
	templatesPath := filepath.Join("web", "templates", "*.html")
	tmpl, err := template.ParseGlob(templatesPath)
	if err != nil {
		return nil, err
	}

	return &WebHandler{
		sessionMgr: sessionMgr,
		clientMgr:  clientMgr,
		config:     config,
		templates:  tmpl,
	}, nil
}

// requireAuth middleware to check if user is authenticated
func (wh *WebHandler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		session, exists := wh.sessionMgr.GetSession(cookie.Value)
		if !exists {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Refresh session
		wh.sessionMgr.RefreshSession(session.ID)

		next(w, r)
	}
}

// HandleLogin serves the login page
func (wh *WebHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if cookie, err := r.Cookie("session_id"); err == nil {
		if _, exists := wh.sessionMgr.GetSession(cookie.Value); exists {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}

	if err := wh.templates.ExecuteTemplate(w, "login.html", nil); err != nil {
		log.Printf("Error rendering login template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleLoginAPI processes login requests
func (wh *WebHandler) HandleLoginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate credentials
	if credentials.Username != wh.config.Username || credentials.Password != wh.config.Password {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username or password"})
		return
	}

	// Create session
	session, err := wh.sessionMgr.CreateSession(credentials.Username)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleLogout processes logout requests
func (wh *WebHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_id")
	if err == nil {
		wh.sessionMgr.DeleteSession(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleDashboard serves the dashboard page
func (wh *WebHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if err := wh.templates.ExecuteTemplate(w, "dashboard.html", nil); err != nil {
		log.Printf("Error rendering dashboard template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleTerminalPage serves the terminal page
func (wh *WebHandler) HandleTerminalPage(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	data := struct {
		ClientID string
	}{
		ClientID: clientID,
	}

	if err := wh.templates.ExecuteTemplate(w, "terminal.html", data); err != nil {
		log.Printf("Error rendering terminal template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleFilesPage serves the file manager page
func (wh *WebHandler) HandleFilesPage(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	data := struct {
		ClientID string
	}{
		ClientID: clientID,
	}

	if err := wh.templates.ExecuteTemplate(w, "files.html", data); err != nil {
		log.Printf("Error rendering files template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleClientsAPI returns the list of connected clients (protected by auth)
func (wh *WebHandler) HandleClientsAPI(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	cookie, err := r.Cookie("session_id")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if _, exists := wh.sessionMgr.GetSession(cookie.Value); !exists {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Get clients from original handler
	clients := wh.clientMgr.GetClients()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// HandleFileBrowse handles file browsing requests
func (wh *WebHandler) HandleFileBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID string `json:"client_id"`
		Path     string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get client
	client, ok := wh.clientMgr.GetClient(req.ClientID)
	if !ok || client == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Clear any previous result
	wh.server.ClearFileListResult(req.ClientID)

	// Send file browse request
	msg, err := common.NewMessage(common.MsgTypeBrowseFiles, common.BrowseFilesPayload{
		Path: req.Path,
	})
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(req.ClientID, msg); err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	// Wait for response with timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			http.Error(w, "Request timeout", http.StatusRequestTimeout)
			return
		case <-ticker.C:
			if result, exists := wh.server.GetFileListResult(req.ClientID); exists {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
				wh.server.ClearFileListResult(req.ClientID)
				return
			}
		}
	}
}

// HandleFileDownload handles file download requests
func (wh *WebHandler) HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID string `json:"client_id"`
		Path     string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get client
	client, ok := wh.clientMgr.GetClient(req.ClientID)
	if !ok || client == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Send file download request
	msg, err := common.NewMessage(common.MsgTypeDownloadFile, common.FileDataPayload{
		Path: req.Path,
	})
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(req.ClientID, msg); err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	// Note: For file downloads, we'd need to implement a chunked response system
	// For now, acknowledge the request was sent
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "download_initiated"})
}

// HandleGlobalUpdate handles global update requests for all clients
func (wh *WebHandler) HandleGlobalUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version  string            `json:"version"`
		URLs     map[string]string `json:"urls"`      // platform -> URL mapping
		Checksum map[string]string `json:"checksums"` // platform -> checksum mapping
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if req.Version == "" {
		http.Error(w, "Version is required", http.StatusBadRequest)
		return
	}

	if len(req.URLs) == 0 {
		http.Error(w, "At least one platform URL is required", http.StatusBadRequest)
		return
	}

	log.Printf("Global update initiated: version=%s, platforms=%d", req.Version, len(req.URLs))

	// Get all online clients
	clients := wh.clientMgr.GetClients()
	onlineClients := []*common.ClientMetadata{}
	for _, client := range clients {
		if client.Status == "online" {
			onlineClients = append(onlineClients, client)
		}
	}

	if len(onlineClients) == 0 {
		http.Error(w, "No online clients to update", http.StatusBadRequest)
		return
	}

	// Send platform-specific update to each client
	successCount := 0
	failCount := 0
	skippedCount := 0
	platformStats := make(map[string]int)

	for _, client := range onlineClients {
		// Build platform identifier (e.g., "windows/amd64", "linux/amd64")
		platform := client.OS + "/" + client.Arch
		platformStats[platform]++

		// Get URL for this platform
		downloadURL, hasURL := req.URLs[platform]
		if !hasURL {
			log.Printf("No URL provided for platform %s, skipping client %s", platform, client.ID)
			skippedCount++
			continue
		}

		// Get checksum for this platform (optional)
		checksum := ""
		if req.Checksum != nil {
			checksum = req.Checksum[platform]
		}

		// Create platform-specific update payload
		updatePayload := common.UpdatePayload{
			Version:     req.Version,
			DownloadURL: downloadURL,
			Checksum:    checksum,
		}

		msg, err := common.NewMessage(common.MsgTypeUpdate, updatePayload)
		if err != nil {
			log.Printf("Failed to create message for client %s: %v", client.ID, err)
			failCount++
			continue
		}

		if err := wh.clientMgr.SendToClient(client.ID, msg); err != nil {
			log.Printf("Failed to send update to client %s (%s): %v", client.ID, platform, err)
			failCount++
		} else {
			log.Printf("Update sent to client %s (%s)", client.ID, platform)
			successCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"total_clients":  len(onlineClients),
		"success_count":  successCount,
		"fail_count":     failCount,
		"skipped_count":  skippedCount,
		"version":        req.Version,
		"platform_stats": platformStats,
		"message":        "Update command sent to online clients",
	})
}

// RegisterWebRoutes registers all web UI routes
func (wh *WebHandler) RegisterWebRoutes(mux *http.ServeMux) {
	// Public routes
	mux.HandleFunc("/login", wh.HandleLogin)
	mux.HandleFunc("/api/login", wh.HandleLoginAPI)
	mux.HandleFunc("/api/logout", wh.HandleLogout)

	// Protected routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/dashboard", wh.requireAuth(wh.HandleDashboard))
	mux.HandleFunc("/terminal", wh.requireAuth(wh.HandleTerminalPage))
	mux.HandleFunc("/files", wh.requireAuth(wh.HandleFilesPage))
	mux.HandleFunc("/api/files/browse", wh.requireAuth(wh.HandleFileBrowse))
	mux.HandleFunc("/api/files/download", wh.requireAuth(wh.HandleFileDownload))
	mux.HandleFunc("/api/screenshot", wh.requireAuth(wh.HandleScreenshotRequest))
	mux.HandleFunc("/api/update/global", wh.requireAuth(wh.HandleGlobalUpdate))
}
