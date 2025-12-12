package messaging

import (
"testing"
"time"

"gorat/pkg/protocol"
)

// MockResultStore implements ResultStore for testing
type MockResultStore struct {
	commandResults   map[string]*protocol.CommandResultPayload
	fileListResults  map[string]*protocol.FileListPayload
	driveListResults map[string]*protocol.DriveListPayload
	processResults   map[string]*protocol.ProcessListPayload
	systemResults    map[string]*protocol.SystemInfoPayload
	fileDataResults  map[string]*protocol.FileDataPayload
	screenshotResults map[string]*protocol.ScreenshotDataPayload
}

func NewMockResultStore() *MockResultStore {
	return &MockResultStore{
		commandResults:    make(map[string]*protocol.CommandResultPayload),
		fileListResults:   make(map[string]*protocol.FileListPayload),
		driveListResults:  make(map[string]*protocol.DriveListPayload),
		processResults:    make(map[string]*protocol.ProcessListPayload),
		systemResults:     make(map[string]*protocol.SystemInfoPayload),
		fileDataResults:   make(map[string]*protocol.FileDataPayload),
		screenshotResults: make(map[string]*protocol.ScreenshotDataPayload),
	}
}

func (m *MockResultStore) SetCommandResult(clientID string, result *protocol.CommandResultPayload) {
	m.commandResults[clientID] = result
}

func (m *MockResultStore) GetCommandResult(clientID string) *protocol.CommandResultPayload {
	return m.commandResults[clientID]
}

func (m *MockResultStore) SetFileListResult(clientID string, result *protocol.FileListPayload) {
	m.fileListResults[clientID] = result
}

func (m *MockResultStore) GetFileListResult(clientID string) *protocol.FileListPayload {
	return m.fileListResults[clientID]
}

func (m *MockResultStore) SetDriveListResult(clientID string, result *protocol.DriveListPayload) {
	m.driveListResults[clientID] = result
}

func (m *MockResultStore) GetDriveListResult(clientID string) *protocol.DriveListPayload {
	return m.driveListResults[clientID]
}

func (m *MockResultStore) SetProcessListResult(clientID string, result *protocol.ProcessListPayload) {
	m.processResults[clientID] = result
}

func (m *MockResultStore) GetProcessListResult(clientID string) *protocol.ProcessListPayload {
	return m.processResults[clientID]
}

func (m *MockResultStore) SetSystemInfoResult(clientID string, result *protocol.SystemInfoPayload) {
	m.systemResults[clientID] = result
}

func (m *MockResultStore) GetSystemInfoResult(clientID string) *protocol.SystemInfoPayload {
	return m.systemResults[clientID]
}

func (m *MockResultStore) SetFileDataResult(clientID string, result *protocol.FileDataPayload) {
	m.fileDataResults[clientID] = result
}

func (m *MockResultStore) GetFileDataResult(clientID string) *protocol.FileDataPayload {
	return m.fileDataResults[clientID]
}

func (m *MockResultStore) SetScreenshotResult(clientID string, result *protocol.ScreenshotDataPayload) {
	m.screenshotResults[clientID] = result
}

func (m *MockResultStore) GetScreenshotResult(clientID string) *protocol.ScreenshotDataPayload {
	return m.screenshotResults[clientID]
}

// MockClientMetadataUpdater implements ClientMetadataUpdater for testing
type MockClientMetadataUpdater struct {
	metadata map[string]*protocol.ClientMetadata
}

func NewMockClientMetadataUpdater() *MockClientMetadataUpdater {
	return &MockClientMetadataUpdater{
		metadata: make(map[string]*protocol.ClientMetadata),
	}
}

func (m *MockClientMetadataUpdater) UpdateClientMetadata(clientID string, fn func(*protocol.ClientMetadata)) {
	if _, exists := m.metadata[clientID]; !exists {
		m.metadata[clientID] = &protocol.ClientMetadata{ID: clientID}
	}
	fn(m.metadata[clientID])
}

func (m *MockClientMetadataUpdater) GetMetadata(clientID string) *protocol.ClientMetadata {
	return m.metadata[clientID]
}

// Tests

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	if d == nil {
		t.Fatal("Dispatcher should not be nil")
	}
}

func TestRegisterHandler(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	handler := NewCommandResultHandler(store)

	err := d.Register(handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	if !d.HasHandler(protocol.MsgTypeCommandResult) {
		t.Fatal("Handler should be registered")
	}
}

func TestRegisterDuplicateHandler(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	handler := NewCommandResultHandler(store)

	d.Register(handler)
	err := d.Register(handler)

	if err == nil {
		t.Fatal("Should not allow duplicate handler registration")
	}
}

func TestDispatchCommandResult(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	handler := NewCommandResultHandler(store)
	d.Register(handler)

	payload := protocol.CommandResultPayload{
		Success:  true,
		Output:   "test output",
		ExitCode: 0,
	}

	msg, _ := protocol.NewMessage(protocol.MsgTypeCommandResult, payload)
	_, err := d.Dispatch("client1", msg)

	if err != nil {
		t.Fatalf("Failed to dispatch message: %v", err)
	}

	result := store.GetCommandResult("client1")
	if result == nil {
		t.Fatal("Result should be stored")
	}

	if result.Output != "test output" {
		t.Errorf("Expected output 'test output', got '%s'", result.Output)
	}
}

func TestDispatchHeartbeat(t *testing.T) {
	d := NewDispatcher()
	updater := NewMockClientMetadataUpdater()
	handler := NewHeartbeatHandler(updater)
	d.Register(handler)

	payload := protocol.HeartbeatPayload{
		ClientID:   "client1",
		Status:     "online",
		CPUUsage:   45.5,
		MemUsage:   60.2,
		LastActive: time.Now(),
	}

	msg, _ := protocol.NewMessage(protocol.MsgTypeHeartbeat, payload)
	_, err := d.Dispatch("client1", msg)

	if err != nil {
		t.Fatalf("Failed to dispatch heartbeat: %v", err)
	}

	metadata := updater.GetMetadata("client1")
	if metadata == nil {
		t.Fatal("Metadata should be updated")
	}

	if metadata.Status != "online" {
		t.Errorf("Expected status 'online', got '%s'", metadata.Status)
	}
}

func TestDispatchFileList(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	handler := NewFileListHandler(store)
	d.Register(handler)

	payload := protocol.FileListPayload{
		Path: "/test",
		Files: []protocol.FileInfo{
			{Name: "file1.txt", Path: "/test/file1.txt", Size: 100},
			{Name: "file2.txt", Path: "/test/file2.txt", Size: 200},
		},
	}

	msg, _ := protocol.NewMessage(protocol.MsgTypeFileList, payload)
	_, err := d.Dispatch("client1", msg)

	if err != nil {
		t.Fatalf("Failed to dispatch file list: %v", err)
	}

	result := store.GetFileListResult("client1")
	if result == nil {
		t.Fatal("Result should be stored")
	}

	if len(result.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(result.Files))
	}
}

func TestDispatchScreenshot(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	handler := NewScreenshotDataHandler(store)
	d.Register(handler)

	payload := protocol.ScreenshotDataPayload{
		Data:      []byte{0x1, 0x2, 0x3},
		Format:    "png",
		Width:     1024,
		Height:    768,
		Timestamp: time.Now(),
	}

	msg, _ := protocol.NewMessage(protocol.MsgTypeScreenshotData, payload)
	_, err := d.Dispatch("client1", msg)

	if err != nil {
		t.Fatalf("Failed to dispatch screenshot: %v", err)
	}

	result := store.GetScreenshotResult("client1")
	if result == nil {
		t.Fatal("Result should be stored")
	}

	if result.Width != 1024 {
		t.Errorf("Expected width 1024, got %d", result.Width)
	}
}

func TestDispatchUnknownHandler(t *testing.T) {
	d := NewDispatcher()

	msg, _ := protocol.NewMessage(protocol.MsgTypeCommandResult, nil)
	_, err := d.Dispatch("client1", msg)

	if err == nil {
		t.Fatal("Should return error for unregistered handler")
	}
}

func TestHasHandler(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	handler := NewCommandResultHandler(store)
	d.Register(handler)

	if !d.HasHandler(protocol.MsgTypeCommandResult) {
		t.Fatal("HasHandler should return true for registered handler")
	}

	if d.HasHandler(protocol.MsgTypeFileList) {
		t.Fatal("HasHandler should return false for unregistered handler")
	}
}

func TestMultipleHandlers(t *testing.T) {
	d := NewDispatcher()
	store := NewMockResultStore()
	updater := NewMockClientMetadataUpdater()

	d.Register(NewCommandResultHandler(store))
	d.Register(NewFileListHandler(store))
	d.Register(NewHeartbeatHandler(updater))

	if d.HasHandler(protocol.MsgTypeCommandResult) &&
		d.HasHandler(protocol.MsgTypeFileList) &&
		d.HasHandler(protocol.MsgTypeHeartbeat) {
		// All handlers registered successfully
	} else {
		t.Fatal("All handlers should be registered")
	}
}
