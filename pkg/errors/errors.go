package errors

import (
	"errors"
	"fmt"
)

// Authentication errors
var (
	// ErrAuthFailed is returned when authentication fails
	ErrAuthFailed = errors.New("authentication failed")

	// ErrInvalidToken is returned when token is invalid
	ErrInvalidToken = errors.New("invalid token")

	// ErrUnauthorized is returned when access is denied
	ErrUnauthorized = errors.New("unauthorized")
)

// Client management errors
var (
	// ErrClientNotFound is returned when a client is not found
	ErrClientNotFound = errors.New("client not found")

	// ErrSendTimeout is returned when sending to a client times out
	ErrSendTimeout = errors.New("send timeout")

	// ErrNotConnected is returned when client is not connected
	ErrNotConnected = errors.New("not connected to server")

	// ErrClientDisconnected is returned when client disconnects
	ErrClientDisconnected = errors.New("client disconnected")
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

	// ErrRecordNotFound is returned when a record is not found
	ErrRecordNotFound = errors.New("record not found")
)

// Configuration errors
var (
	// ErrConfigNotFound is returned when configuration file is not found
	ErrConfigNotFound = errors.New("configuration not found")

	// ErrInvalidConfig is returned when configuration is invalid
	ErrInvalidConfig = errors.New("invalid configuration")
)

// Path validation errors
var (
	// ErrPathTraversal is returned when path traversal is detected
	ErrPathTraversal = errors.New("path traversal attack detected")

	// ErrInvalidPath is returned when path is invalid
	ErrInvalidPath = errors.New("invalid path")
)

// Timeout errors
var (
	// ErrTimeout is returned on timeout
	ErrTimeout = errors.New("operation timeout")
)

// ErrorWithContext wraps an error with additional context
type ErrorWithContext struct {
	Err     error
	Message string
	Context map[string]interface{}
}

// Error implements the error interface
func (e *ErrorWithContext) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *ErrorWithContext) Unwrap() error {
	return e.Err
}

// Wrap wraps an error with a message
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return &ErrorWithContext{
		Err:     err,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to an error
func WithContext(err error, key string, value interface{}) error {
	if ewc, ok := err.(*ErrorWithContext); ok {
		if ewc.Context == nil {
			ewc.Context = make(map[string]interface{})
		}
		ewc.Context[key] = value
		return ewc
	}
	ewc := &ErrorWithContext{
		Err:     err,
		Context: make(map[string]interface{}),
	}
	ewc.Context[key] = value
	return ewc
}
