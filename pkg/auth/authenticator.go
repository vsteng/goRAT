package auth

import (
	"log"

	"gorat/common"
)

// AuthenticatorImpl implements Authenticator interface
type AuthenticatorImpl struct {
	serverToken string
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(serverToken string) Authenticator {
	return &AuthenticatorImpl{
		serverToken: serverToken,
	}
}

// Authenticate authenticates a client with a token
func (a *AuthenticatorImpl) Authenticate(payload *common.AuthPayload) (bool, string) {
	// Validate token
	if payload.Token != a.serverToken {
		log.Printf("Authentication failed for client %s: invalid token", payload.ClientID)
		return false, ""
	}

	// Generate new token for this session
	token := common.GenerateToken(payload.ClientID)
	log.Printf("Client %s authenticated successfully", payload.ClientID)

	return true, token
}
