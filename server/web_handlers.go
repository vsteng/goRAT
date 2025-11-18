package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
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
}
