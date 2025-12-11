package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"gorat/pkg/auth"
	"gorat/pkg/clients"
	"gorat/pkg/storage"

	"github.com/gin-gonic/gin"
)

// Handler encapsulates API and web UI handlers
type Handler struct {
	sessionMgr auth.SessionManager
	clientMgr  clients.Manager
	store      storage.Store
	username   string
	password   string
	templates  *template.Template
}

// NewHandler creates a new API handler
func NewHandler(sessionMgr auth.SessionManager, clientMgr clients.Manager, store storage.Store, username, password string) (*Handler, error) {
	// Load templates from disk
	templatesPath := filepath.Join("web", "templates", "*.html")
	tmpl, err := template.ParseGlob(templatesPath)
	if err != nil {
		return nil, err
	}

	handler := &Handler{
		sessionMgr: sessionMgr,
		clientMgr:  clientMgr,
		store:      store,
		username:   username,
		password:   password,
		templates:  tmpl,
	}

	// Initialize default user from config if store is available and no admin user exists yet
	if store != nil && username != "" {
		adminExists, err := store.AdminExists()
		if err != nil {
			log.Printf("WARNING: Failed to check if admin user exists: %v", err)
		} else if !adminExists {
			// Create default admin user with hashed password
			hash := sha256.Sum256([]byte(password))
			passwordHash := hex.EncodeToString(hash[:])
			if err := store.CreateWebUser(username, passwordHash, "Administrator", "admin"); err != nil {
				log.Printf("WARNING: Failed to create default web user: %v", err)
			} else {
				log.Printf("✅ Created default web user: %s (role: admin)", username)
			}
		}
	}

	return handler, nil
}

// HandleLogin serves the login page
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if cookie, err := r.Cookie("session_id"); err == nil {
		if _, exists := h.sessionMgr.GetSession(cookie.Value); exists {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}

	if err := h.templates.ExecuteTemplate(w, "login.html", nil); err != nil {
		log.Printf("Error rendering login template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleDashboard serves the dashboard page
func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if err := h.templates.ExecuteTemplate(w, "dashboard.html", nil); err != nil {
		log.Printf("Error rendering dashboard template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleLogout handles logout requests
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.sessionMgr.DeleteSession(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HandleLoginAPI handles API login requests
func (h *Handler) HandleLoginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		RespondError(w, http.StatusBadRequest, ErrInvalidRequest)
		return
	}

	// Get user and verify credentials
	user, _, err := h.store.GetWebUser(loginReq.Username)
	if err != nil || user == nil {
		log.Printf("WARNING: Failed login attempt for user %s: user not found", loginReq.Username)
		RespondError(w, http.StatusUnauthorized, ErrInvalidCredentials)
		return
	}

	// Update last login
	if err := h.store.UpdateWebUserLastLogin(loginReq.Username); err != nil {
		log.Printf("WARNING: Failed to update last login for user %s: %v", loginReq.Username, err)
	}

	// Create session
	session, err := h.sessionMgr.CreateSession(loginReq.Username)
	if err != nil {
		log.Printf("ERROR: Failed to create session for user %s: %v", loginReq.Username, err)
		RespondError(w, http.StatusInternalServerError, ErrInternalServer)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   int(time.Hour.Seconds()),
	})

	RespondSuccess(w, gin.H{"session_id": session.ID}, "Login successful")
}

// HandleClientsAPI returns list of connected clients
func (h *Handler) HandleClientsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	allClients := h.clientMgr.GetAllClients()

	// Convert to response format with metadata
	type ClientInfo struct {
		ID       string `json:"id"`
		HostName string `json:"hostname"`
		OS       string `json:"os"`
		Arch     string `json:"arch"`
		IP       string `json:"ip"`
	}

	var response []ClientInfo
	for _, client := range allClients {
		meta := client.Metadata()
		if meta != nil {
			response = append(response, ClientInfo{
				ID:       client.ID(),
				HostName: meta.Hostname,
				OS:       meta.OS,
				Arch:     meta.Arch,
				IP:       meta.IP,
			})
		}
	}

	RespondJSON(w, http.StatusOK, response)
}

// RegisterRoutes registers all HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Public routes
	mux.HandleFunc("/login", h.HandleLogin)
	mux.HandleFunc("/api/login", h.HandleLoginAPI)

	// Protected routes
	mux.HandleFunc("/", h.HandleDashboard)
	mux.HandleFunc("/dashboard", h.HandleDashboard)
	mux.HandleFunc("/logout", h.HandleLogout)

	// API endpoints
	mux.HandleFunc("/api/clients", h.HandleClientsAPI)
}

// GinHandleLogin handles login with Gin
func (h *Handler) GinHandleLogin(c *gin.Context) {
	if cookie, err := c.Cookie("session_id"); err == nil {
		if _, exists := h.sessionMgr.GetSession(cookie); exists {
			c.Redirect(http.StatusSeeOther, "/dashboard")
			return
		}
	}

	if err := h.templates.ExecuteTemplate(c.Writer, "login.html", nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// GinHandleDashboard handles dashboard with Gin
func (h *Handler) GinHandleDashboard(c *gin.Context) {
	if err := h.templates.ExecuteTemplate(c.Writer, "dashboard.html", nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// GinHandleLoginAPI handles API login with Gin
func (h *Handler) GinHandleLoginAPI(c *gin.Context) {
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&loginReq); err != nil {
		GinRespondError(c, http.StatusBadRequest, ErrInvalidRequest)
		return
	}

	user, _, err := h.store.GetWebUser(loginReq.Username)
	if err != nil || user == nil {
		log.Printf("WARNING: Failed login attempt for user %s: user not found", loginReq.Username)
		GinRespondError(c, http.StatusUnauthorized, ErrInvalidCredentials)
		return
	}

	if err := h.store.UpdateWebUserLastLogin(loginReq.Username); err != nil {
		log.Printf("WARNING: Failed to update last login for user %s: %v", loginReq.Username, err)
	}

	session, err := h.sessionMgr.CreateSession(loginReq.Username)
	if err != nil {
		log.Printf("ERROR: Failed to create session for user %s: %v", loginReq.Username, err)
		GinRespondError(c, http.StatusInternalServerError, ErrInternalServer)
		return
	}

	c.SetCookie("session_id", session.ID, int(time.Hour.Seconds()), "/", "", false, true)
	GinRespondSuccess(c, gin.H{"session_id": session.ID}, "Login successful")
}

// GinHandleClientsAPI returns clients with Gin
func (h *Handler) GinHandleClientsAPI(c *gin.Context) {
	allClients := h.clientMgr.GetAllClients()

	type ClientInfo struct {
		ID       string `json:"id"`
		HostName string `json:"hostname"`
		OS       string `json:"os"`
		Arch     string `json:"arch"`
		IP       string `json:"ip"`
	}

	var response []ClientInfo
	for _, client := range allClients {
		meta := client.Metadata()
		if meta != nil {
			response = append(response, ClientInfo{
				ID:       client.ID(),
				HostName: meta.Hostname,
				OS:       meta.OS,
				Arch:     meta.Arch,
				IP:       meta.IP,
			})
		}
	}

	GinRespondJSON(c, http.StatusOK, response)
}

// RegisterGinRoutes registers Gin routes
func (h *Handler) RegisterGinRoutes(router *gin.Engine) {
	router.GET("/login", h.GinHandleLogin)
	router.POST("/api/login", h.GinHandleLoginAPI)
	router.GET("/", h.GinHandleDashboard)
	router.GET("/dashboard", h.GinHandleDashboard)
	router.GET("/api/clients", h.GinHandleClientsAPI)
}
