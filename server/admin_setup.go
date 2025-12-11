package server

import (
	"log"

	ggin "github.com/gin-gonic/gin"
)

// SetupGinRouter initializes the Gin router with your existing handlers
// This maintains all your WebSocket and API functionality while adding a modern framework
func SetupGinRouter(config *Config, manager *ClientManager, store *ClientStore, sessionMgr *SessionManager, terminalProxy *TerminalProxy) (*ggin.Engine, error) {
	// Create Gin engine
	router := ggin.Default()

	// Enable CORS for cross-origin requests
	router.Use(CORSMiddleware())

	// Static files
	router.Static("/static", "./web/static")
	router.Static("/assets", "./web/assets")

	// Load HTML templates
	router.LoadHTMLGlob("web/templates/*.html")

	log.Println("✅ Gin router initialized")
	return router, nil
}

// CORSMiddleware handles CORS headers
func CORSMiddleware() ggin.HandlerFunc {
	return func(c *ggin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
