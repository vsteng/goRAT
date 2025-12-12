package errors

import "errors"

// Authentication errors
var (
	// ErrAuthFailed is returned when authentication fails
	ErrAuthFailed = errors.New("authentication failed")

	// ErrInvalidToken is returned when token is invalid
	ErrInvalidToken = errors.New("invalid token")
)

// Client management errors
var (
	// ErrClientNotFound is returned when a client is not found
	ErrClientNotFound = errors.New("client not found")

	// ErrSendTimeout is returned when sending to a client times out
	ErrSendTimeout = errors.New("send timeout")

	// ErrNotConnected is returned when client is not connected
	ErrNotConnected = errors.New("not connected to server")
)

// Message and protocol errors
var (
	// ErrInvalidMessage is returned when a message is invalid
	ErrInvalidMessage = errors.New("invalid message")

	// ErrInvalidResponse is returned when server response is invalid
	ErrInvalidResponse = errors.New("invalid server response")
)

// Storage errors
var (
	// ErrStorageNotInitialized is returned when storage is not initialized
	ErrStorageNotInitialized = errors.New("storage not initialized")

	// ErrDatabaseConnection is returned when database connection fails
	ErrDatabaseConnection = errors.New("database connection failed")
)

// Configuration errors
var (
	// ErrConfigNotFound is returned when configuration file is not found
	ErrConfigNotFound = errors.New("configuration not found")

	// ErrInvalidConfig is returned when configuration is invalid
	ErrInvalidConfig = errors.New("invalid configuration")
)
