package server

import (
	"log"

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
		log.Printf("Authentication failed for client %s: invalid token", payload.ClientID)
		return false, ""
	}

	// Generate new token for this session
	token := protocol.GenerateToken(payload.ClientID)
	log.Printf("Client %s authenticated successfully", payload.ClientID)

	return true, token
}
