package messaging

import (
"gorat/pkg/protocol"
)

// Handler handles a specific message type
type Handler interface {
	// Handle processes a message and returns an optional response
	Handle(clientID string, msg *protocol.Message) (interface{}, error)
	// MessageType returns the type of message this handler processes
	MessageType() protocol.MessageType
}

// Dispatcher dispatches messages to appropriate handlers
type Dispatcher interface {
	// Register registers a handler for a message type
	Register(handler Handler) error
	// Dispatch dispatches a message to the appropriate handler
	Dispatch(clientID string, msg *protocol.Message) (interface{}, error)
	// HasHandler checks if a handler exists for the message type
	HasHandler(msgType protocol.MessageType) bool
}

// ResultStore stores command results, file listings, etc.
type ResultStore interface {
	// SetCommandResult stores a command result
	SetCommandResult(clientID string, result *protocol.CommandResultPayload)
	// GetCommandResult retrieves a command result
	GetCommandResult(clientID string) *protocol.CommandResultPayload
	// SetFileListResult stores a file list result
	SetFileListResult(clientID string, result *protocol.FileListPayload)
	// GetFileListResult retrieves a file list result
	GetFileListResult(clientID string) *protocol.FileListPayload
	// SetDriveListResult stores a drive list result
	SetDriveListResult(clientID string, result *protocol.DriveListPayload)
	// GetDriveListResult retrieves a drive list result
	GetDriveListResult(clientID string) *protocol.DriveListPayload
	// SetProcessListResult stores a process list result
	SetProcessListResult(clientID string, result *protocol.ProcessListPayload)
	// GetProcessListResult retrieves a process list result
	GetProcessListResult(clientID string) *protocol.ProcessListPayload
	// SetSystemInfoResult stores a system info result
	SetSystemInfoResult(clientID string, result *protocol.SystemInfoPayload)
	// GetSystemInfoResult retrieves a system info result
	GetSystemInfoResult(clientID string) *protocol.SystemInfoPayload
	// SetFileDataResult stores a file data result
	SetFileDataResult(clientID string, result *protocol.FileDataPayload)
	// GetFileDataResult retrieves a file data result
	GetFileDataResult(clientID string) *protocol.FileDataPayload
	// SetScreenshotResult stores a screenshot result
	SetScreenshotResult(clientID string, result *protocol.ScreenshotDataPayload)
	// GetScreenshotResult retrieves a screenshot result
	GetScreenshotResult(clientID string) *protocol.ScreenshotDataPayload
}

// ClientMetadataUpdater updates client metadata
type ClientMetadataUpdater interface {
	// UpdateClientMetadata updates client metadata with the given function
	UpdateClientMetadata(clientID string, fn func(*protocol.ClientMetadata))
}
