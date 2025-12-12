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
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
