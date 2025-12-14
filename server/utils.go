package server

import (
	"gorat/pkg/logger"
	"gorat/pkg/protocol"
)

// Authenticator handles client authentication
type Authenticator struct {
	serverToken string
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(serverToken string) *Authenticator {
	return &Authenticator{
		serverToken: serverToken,
	}
}

// Authenticate authenticates a client
func (a *Authenticator) Authenticate(payload *protocol.AuthPayload) (bool, string) {
	// Validate token
	if payload.Token != a.serverToken {
		logger.Get().WarnWith("authentication failed: invalid token", "clientID", payload.ClientID)
		return false, ""
	}

	// Generate new token for this session
	token := protocol.GenerateToken(payload.ClientID)
	logger.Get().DebugWith("client authenticated successfully", "clientID", payload.ClientID)

	return true, token
}
