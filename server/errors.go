package server

import "errors"

var (
	// ErrClientNotFound is returned when a client is not found
	ErrClientNotFound = errors.New("client not found")

	// ErrSendTimeout is returned when sending to a client times out
	ErrSendTimeout = errors.New("send timeout")

	// ErrAuthFailed is returned when authentication fails
	ErrAuthFailed = errors.New("authentication failed")

	// ErrInvalidMessage is returned when a message is invalid
	ErrInvalidMessage = errors.New("invalid message")
)
