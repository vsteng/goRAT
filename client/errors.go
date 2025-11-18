package client

import "errors"

var (
	// ErrAuthFailed is returned when authentication fails
	ErrAuthFailed = errors.New("authentication failed")

	// ErrInvalidResponse is returned when server response is invalid
	ErrInvalidResponse = errors.New("invalid server response")

	// ErrNotConnected is returned when client is not connected
	ErrNotConnected = errors.New("not connected to server")
)
