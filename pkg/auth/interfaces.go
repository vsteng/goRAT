package auth

import (
	"time"

	"gorat/pkg/protocol"
)

// SessionManager defines the interface for web session management
type SessionManager interface {
	// CreateSession creates a new session for a user
	CreateSession(username string) (*Session, error)

	// GetSession retrieves a session by ID
	GetSession(sessionID string) (*Session, bool)

	// RefreshSession extends the expiration time of a session
	RefreshSession(sessionID string) bool

	// DeleteSession removes a session
	DeleteSession(sessionID string)

	// GetAllSessions returns all active sessions
	GetAllSessions() []*Session

	// UpdateSessionContext updates session with client IP and User-Agent
	UpdateSessionContext(sessionID, clientIP, userAgent string) bool

	// VerifySessionContext checks if session's IP and User-Agent match request
	VerifySessionContext(sessionID, clientIP, userAgent string) bool
}

// Authenticator defines the interface for client authentication
type Authenticator interface {
	// Authenticate authenticates a client with a token
	Authenticate(payload *protocol.AuthPayload) (bool, string)
}

// Session represents a web session
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
	ClientIP  string // Client IP address for security verification
	UserAgent string // User agent for security verification
	Verified  bool   // Has session been verified after initial creation
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValidForRequest checks if session is valid for the current request context
func (s *Session) IsValidForRequest(clientIP, userAgent string) bool {
	// After initial verification, enforce matching IP and User-Agent
	if s.Verified {
		return s.ClientIP == clientIP && s.UserAgent == userAgent
	}
	// First request: allow if user agent matches (IP may change during session)
	return s.UserAgent == userAgent
}
