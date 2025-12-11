// Package auth provides authentication and session management for goRAT.
//
// This package includes:
// - SessionManager: Manages web UI sessions with automatic expiration
// - Authenticator: Handles client authentication with token validation
//
// Usage:
//
//	sessionMgr := auth.NewSessionManager(24 * time.Hour)
//	authenticator := auth.NewAuthenticator(serverToken)
//
//	// Create a session
//	session, err := sessionMgr.CreateSession("username")
//
//	// Authenticate a client
//	success, token := authenticator.Authenticate(payload)
//
// The interfaces allow for alternative implementations such as
// OAuth, JWT, or external authentication providers.
package auth
