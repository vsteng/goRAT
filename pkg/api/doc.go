// Package api provides HTTP API handlers and middleware for the server.
//
// This package encapsulates all HTTP-related concerns:
// - REST API endpoints for clients, proxies, users
// - Web UI handlers (dashboard, file browser, terminal)
// - Authentication middleware
// - Error responses
// - CORS and other HTTP middleware
//
// The package uses gin-gonic for routing but provides abstractions
// that allow the server to use either standard net/http or gin.
package api
