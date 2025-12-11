package api

import (
	"net/http"

	"gorat/pkg/auth"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware is HTTP handler middleware for standard net/http
func AuthMiddleware(sessionMgr auth.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		session, exists := sessionMgr.GetSession(cookie.Value)
		if !exists {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Refresh session
		sessionMgr.RefreshSession(session.ID)

		next.ServeHTTP(w, r)
	})
}

// GinAuthMiddleware is Gin middleware for authentication
func GinAuthMiddleware(sessionMgr auth.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("session_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		session, exists := sessionMgr.GetSession(cookie)
		if !exists {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Refresh session
		sessionMgr.RefreshSession(session.ID)

		c.Next()
	}
}

// CORSMiddleware handles CORS headers for Gin
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SetupGinRouter initializes the Gin router with API and web UI routes
func SetupGinRouter(sessionMgr auth.SessionManager) *gin.Engine {
	router := gin.Default()

	// Enable CORS
	router.Use(CORSMiddleware())

	// Static files
	router.Static("/static", "./web/static")
	router.Static("/assets", "./web/assets")

	// Load HTML templates
	router.LoadHTMLGlob("web/templates/*.html")

	return router
}
