package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"mww2.com/server_manager/common"

	"github.com/gin-gonic/gin"
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
	store      *ClientStore
	config     *WebConfig
	templates  *template.Template
	server     *Server // Reference to main server for result access
}

// NewWebHandler creates a new web handler
func NewWebHandler(sessionMgr *SessionManager, clientMgr *ClientManager, store *ClientStore, config *WebConfig) (*WebHandler, error) {
	// Load templates from disk
	templatesPath := filepath.Join("web", "templates", "*.html")
	tmpl, err := template.ParseGlob(templatesPath)
	if err != nil {
		return nil, err
	}

	handler := &WebHandler{
		sessionMgr: sessionMgr,
		clientMgr:  clientMgr,
		store:      store,
		config:     config,
		templates:  tmpl,
	}

	// Initialize default user from config if store is available and no admin user exists yet
	if store != nil && config.Username != "" {
		adminExists, err := store.AdminExists()
		if err != nil {
			log.Printf("WARNING: Failed to check if admin user exists: %v", err)
		} else if !adminExists {
			// Create default admin user with hashed password
			hash := sha256.Sum256([]byte(config.Password))
			passwordHash := hex.EncodeToString(hash[:])
			if err := store.CreateWebUser(config.Username, passwordHash, "Administrator", "admin"); err != nil {
				log.Printf("WARNING: Failed to create default web user: %v", err)
			} else {
				log.Printf("✅ Created default web user: %s (role: admin)", config.Username)
			}
		}
	}

	return handler, nil
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

	// Validate credentials against database if store is available
	if wh.store != nil {
		user, passwordHash, err := wh.store.GetWebUser(credentials.Username)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username or password"})
			return
		}

		// Check if user is active
		if user.Status != "active" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "User account is inactive"})
			return
		}

		// Verify password
		hash := sha256.Sum256([]byte(credentials.Password))
		providedHash := hex.EncodeToString(hash[:])
		if providedHash != passwordHash {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username or password"})
			return
		}

		// Update last login
		_ = wh.store.UpdateWebUserLastLogin(credentials.Username)
	} else {
		// Fallback to config credentials if store is not available
		if credentials.Username != wh.config.Username || credentials.Password != wh.config.Password {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username or password"})
			return
		}
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

	// Get client metadata to determine OS
	client, exists := wh.clientMgr.GetClient(clientID)
	clientOS := "linux" // default to linux/unix
	if exists && client != nil {
		client.mu.RLock()
		if client.Metadata != nil && client.Metadata.OS != "" {
			clientOS = client.Metadata.OS
		}
		client.mu.RUnlock()
	}

	data := struct {
		ClientID string
		ClientOS string
	}{
		ClientID: clientID,
		ClientOS: clientOS,
	}

	if err := wh.templates.ExecuteTemplate(w, "files.html", data); err != nil {
		log.Printf("Error rendering files template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleDashboardNew serves the new enhanced dashboard page
func (wh *WebHandler) HandleDashboardNew(w http.ResponseWriter, r *http.Request) {
	if err := wh.templates.ExecuteTemplate(w, "dashboard-new.html", nil); err != nil {
		log.Printf("Error rendering dashboard-new template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleClientDetails serves the client details page
func (wh *WebHandler) HandleClientDetails(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	data := struct {
		ClientID string
	}{
		ClientID: clientID,
	}

	if err := wh.templates.ExecuteTemplate(w, "client-details.html", data); err != nil {
		log.Printf("Error rendering client-details template: %v", err)
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
	log.Printf("[FileBrowse] Received path: '%s' for client '%s'", req.Path, req.ClientID)

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
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				json.NewEncoder(w).Encode(result)
				wh.server.ClearFileListResult(req.ClientID)
				return
			}
		}
	}
}

// HandleGetDrives handles drive listing requests (Windows)
func (wh *WebHandler) HandleGetDrives(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID string `json:"client_id"`
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
	wh.server.ClearDriveListResult(req.ClientID)

	// Send drive list request
	msg, err := common.NewMessage(common.MsgTypeGetDrives, nil)
	if err != nil {
		http.Error(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(req.ClientID, msg); err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	// Wait for response with timeout
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			http.Error(w, "Request timeout", http.StatusRequestTimeout)
			return
		case <-ticker.C:
			if result, exists := wh.server.GetDriveListResult(req.ClientID); exists {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				json.NewEncoder(w).Encode(result)
				wh.server.ClearDriveListResult(req.ClientID)
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

	client, ok := wh.clientMgr.GetClient(req.ClientID)
	if !ok || client == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	wh.server.ClearFileDataResult(req.ClientID)

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

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			http.Error(w, "Request timeout", http.StatusRequestTimeout)
			return
		case <-ticker.C:
			if result, exists := wh.server.GetFileDataResult(req.ClientID); exists {
				if result.Error != "" {
					http.Error(w, result.Error, http.StatusInternalServerError)
					wh.server.ClearFileDataResult(req.ClientID)
					return
				}

				w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(result.Path)+"\"")
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(result.Data)
				wh.server.ClearFileDataResult(req.ClientID)
				return
			}
		}
	}
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

	// User management API routes
	mux.HandleFunc("/api/users", wh.requireAuth(wh.HandleUsersAPI))
	mux.HandleFunc("/api/users/", wh.requireAuth(wh.HandleUserAPI))

	// Protected routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard-new", http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/dashboard", wh.requireAuth(wh.HandleDashboard))
	mux.HandleFunc("/dashboard-new", wh.requireAuth(wh.HandleDashboardNew))
	mux.HandleFunc("/client-details", wh.requireAuth(wh.HandleClientDetails))
	mux.HandleFunc("/terminal", wh.requireAuth(wh.HandleTerminalPage))
	mux.HandleFunc("/files", wh.requireAuth(wh.HandleFilesPage))
	mux.HandleFunc("/api/files/browse", wh.requireAuth(wh.HandleFileBrowse))
	mux.HandleFunc("/api/files/drives", wh.requireAuth(wh.HandleGetDrives))
	mux.HandleFunc("/api/files/download", wh.requireAuth(wh.HandleFileDownload))
	mux.HandleFunc("/api/screenshot", wh.requireAuth(wh.HandleScreenshotRequest))
	mux.HandleFunc("/api/update/global", wh.requireAuth(wh.HandleGlobalUpdate))
}

// HandleUsersAPI handles GET (list users) and POST (create user) requests
func (wh *WebHandler) HandleUsersAPI(w http.ResponseWriter, r *http.Request) {
	if wh.store == nil {
		http.Error(w, "User management not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all users
		users, err := wh.store.GetAllWebUsers()
		if err != nil {
			log.Printf("Error getting users: %v", err)
			http.Error(w, "Failed to get users", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)

	case http.MethodPost:
		// Create new user
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			FullName string `json:"full_name"`
			Role     string `json:"role"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Username and password are required"})
			return
		}

		if len(req.Password) < 6 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 6 characters"})
			return
		}

		// Check if user already exists
		exists, err := wh.store.UserExists(req.Username)
		if err != nil {
			log.Printf("Error checking user existence: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if exists {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Username already exists"})
			return
		}

		// Set default role if not provided
		if req.Role == "" {
			req.Role = "admin"
		}

		// Hash password before storing
		hash := sha256.Sum256([]byte(req.Password))
		passwordHash := hex.EncodeToString(hash[:])

		// Create user
		if err := wh.store.CreateWebUser(req.Username, passwordHash, req.FullName, req.Role); err != nil {
			log.Printf("Error creating user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User created successfully"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleUserAPI handles PUT (update user) and DELETE (delete user) requests
func (wh *WebHandler) HandleUserAPI(w http.ResponseWriter, r *http.Request) {
	if wh.store == nil {
		http.Error(w, "User management not available", http.StatusServiceUnavailable)
		return
	}

	// Extract username from URL path
	username := r.URL.Path[len("/api/users/"):]
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		// Update user (status, role, password, full_name, etc.)
		var req struct {
			Status   string `json:"status"`
			Role     string `json:"role"`
			FullName string `json:"full_name"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Get current user
		user, _, err := wh.store.GetWebUser(username)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
			return
		}

		// Update fields if provided
		if req.Status != "" {
			user.Status = req.Status
		}
		if req.Role != "" {
			user.Role = req.Role
		}
		if req.FullName != "" {
			user.FullName = req.FullName
		}

		// Build update query dynamically
		updateFields := []interface{}{}
		queryParts := []string{}

		if req.Status != "" {
			queryParts = append(queryParts, "status = ?")
			updateFields = append(updateFields, req.Status)
		}
		if req.Role != "" {
			queryParts = append(queryParts, "role = ?")
			updateFields = append(updateFields, req.Role)
		}
		if req.FullName != "" {
			queryParts = append(queryParts, "full_name = ?")
			updateFields = append(updateFields, req.FullName)
		}

		// Handle password update with hashing
		if req.Password != "" {
			if len(req.Password) < 6 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 6 characters"})
				return
			}
			// Hash the new password
			hash := sha256.Sum256([]byte(req.Password))
			passwordHash := hex.EncodeToString(hash[:])
			queryParts = append(queryParts, "password_hash = ?")
			updateFields = append(updateFields, passwordHash)
		}

		// If nothing to update, return error
		if len(queryParts) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "No fields to update"})
			return
		}

		// Add username to the update fields
		updateFields = append(updateFields, username)

		// Build final query
		updateQuery := "UPDATE web_users SET " + strings.Join(queryParts, ", ") + " WHERE username = ?"

		// Update in database
		wh.store.mu.Lock()
		_, err = wh.store.db.Exec(updateQuery, updateFields...)
		wh.store.mu.Unlock()

		if err != nil {
			log.Printf("Error updating user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User updated successfully"})

	case http.MethodDelete:
		// Delete user
		if err := wh.store.DeleteWebUser(username); err != nil {
			log.Printf("Error deleting user: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete user"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User deleted successfully"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RegisterGinRoutes registers web handler routes with Gin router
func (wh *WebHandler) RegisterGinRoutes(router *gin.Engine) {
	// Load HTML templates
	router.LoadHTMLGlob("web/templates/*.html")

	// Static files
	router.Static("/static", "./web/static")
	router.Static("/assets", "./web/assets")

	// Public routes
	router.GET("/login", wh.ginHandleLogin)
	router.POST("/api/login", wh.ginHandleLoginAPI)
	router.POST("/api/logout", wh.ginHandleLogout)

	// User management API routes
	router.GET("/api/users", wh.ginRequireAuth(wh.ginHandleUsersAPI))
	router.POST("/api/users", wh.ginRequireAuth(wh.ginHandleUsersAPI))
	router.PUT("/api/users/:id", wh.ginRequireAuth(wh.ginHandleUserAPI))
	router.DELETE("/api/users/:id", wh.ginRequireAuth(wh.ginHandleUserAPI))

	// Protected routes
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusSeeOther, "/dashboard-new")
	})
	router.GET("/dashboard", wh.ginRequireAuth(wh.ginHandleDashboard))
	router.GET("/dashboard-new", wh.ginRequireAuth(wh.ginHandleDashboardNew))
	router.GET("/client-details", wh.ginRequireAuth(wh.ginHandleClientDetails))
	router.GET("/terminal", wh.ginRequireAuth(wh.ginHandleTerminalPage))
	router.GET("/files", wh.ginRequireAuth(wh.ginHandleFilesPage))
	router.GET("/api/files/browse", wh.ginRequireAuth(wh.ginHandleFileBrowse))
	router.GET("/api/files/drives", wh.ginRequireAuth(wh.ginHandleGetDrives))
	router.GET("/api/files/download", wh.ginRequireAuth(wh.ginHandleFileDownload))
	router.GET("/api/screenshot", wh.ginRequireAuth(wh.ginHandleScreenshotRequest))
	router.POST("/api/update/global", wh.ginRequireAuth(wh.ginHandleGlobalUpdate))
}

// ginRequireAuth is Gin middleware for authentication
func (wh *WebHandler) ginRequireAuth(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("session_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		session, exists := wh.sessionMgr.GetSession(cookie)
		if !exists {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Refresh session
		wh.sessionMgr.RefreshSession(session.ID)

		handler(c)
	}
}

// Gin wrapper handlers
func (wh *WebHandler) ginHandleLogin(c *gin.Context) {
	wh.HandleLogin(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleLoginAPI(c *gin.Context) {
	wh.HandleLoginAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleLogout(c *gin.Context) {
	wh.HandleLogout(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleUsersAPI(c *gin.Context) {
	wh.HandleUsersAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleUserAPI(c *gin.Context) {
	wh.HandleUserAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleDashboard(c *gin.Context) {
	wh.HandleDashboard(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleDashboardNew(c *gin.Context) {
	wh.HandleDashboardNew(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleClientDetails(c *gin.Context) {
	wh.HandleClientDetails(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleTerminalPage(c *gin.Context) {
	wh.HandleTerminalPage(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleFilesPage(c *gin.Context) {
	wh.HandleFilesPage(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleFileBrowse(c *gin.Context) {
	wh.HandleFileBrowse(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleGetDrives(c *gin.Context) {
	wh.HandleGetDrives(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleFileDownload(c *gin.Context) {
	wh.HandleFileDownload(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleScreenshotRequest(c *gin.Context) {
	wh.HandleScreenshotRequest(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleGlobalUpdate(c *gin.Context) {
	wh.HandleGlobalUpdate(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleClientsAPI(c *gin.Context) {
	wh.HandleClientsAPI(c.Writer, c.Request)
}
