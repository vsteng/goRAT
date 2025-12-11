package messaging

import (
"testing"
"time"

"gorat/common"
)

// MockResultStore implements ResultStore for testing
type MockResultStore struct {
	commandResults   map[string]*common.CommandResultPayload
	fileListResults  map[string]*common.FileListPayload
	driveListResults map[string]*common.DriveListPayload
	processResults   map[string]*common.ProcessListPayload
	systemResults    map[string]*common.SystemInfoPayload
	fileDataResults  map[string]*common.FileDataPayload
	screenshotResults map[string]*common.ScreenshotDataPayload
}

func NewMockResultStore() *MockResultStore {
	return &MockResultStore{
		commandResults:    make(map[string]*common.CommandResultPayload),
		fileListResults:   make(map[string]*common.FileListPayload),
		driveListResults:  make(map[string]*common.DriveListPayload),
		processResults:    make(map[string]*common.ProcessListPayload),
		systemResults:     make(map[string]*common.SystemInfoPayload),
		fileDataResults:   make(map[string]*common.FileDataPayload),
		screenshotResults: make(map[string]*common.ScreenshotDataPayload),
	}
}

func (m *MockResultStore) SetCommandResult(clientID string, result *common.CommandResultPayload) {
	m.commandResults[clientID] = result
}

func (m *MockResultStore) GetCommandResult(clientID string) *common.CommandResultPayload {
	return m.commandResults[clientID]
}

func (m *MockResultStore) SetFileListResult(clientID string, result *common.FileListPayload) {
	m.fileListResults[clientID] = result
}

func (m *MockResultStore) GetFileListResult(clientID string) *common.FileListPayload {
	return m.fileListResults[clientID]
}

func (m *MockResultStore) SetDriveListResult(clientID string, result *common.DriveListPayload) {
	m.driveListResults[clientID] = result
}

func (m *MockResultStore) GetDriveListResult(clientID string) *common.DriveListPayload {
	return m.driveListResults[clientID]
}

func (m *MockResultStore) SetProcessListResult(clientID string, result *common.ProcessListPayload) {
	m.processResults[clientID] = result
}

func (m *MockResultStore) GetProcessListResult(clientID string) *common.ProcessListPayload {
	return m.processResults[clientID]
}

func (m *MockResultStore) SetSystemInfoResult(clientID string, result *common.SystemInfoPayload) {
	m.systemResults[clientID] = result
}

func (m *MockResultStore) GetSystemInfoResult(clientID string) *common.SystemInfoPayload {
	return m.systemResults[clientID]
}

func (m *MockResultStore) SetFileDataResult(clientID string, result *common.FileDataPayload) {
	m.fileDataResults[clientID] = result
}

func (m *MockResultStore) GetFileDataResult(clientID string) *common.FileDataPayload {
	return m.fileDataResults[clientID]
}

func (m *MockResultStore) SetScreenshotResult(clientID string, result *common.ScreenshotDataPayload) {
	m.screenshotResults[clientID] = result
}

func (m *MockResultStore) GetScreenshotResult(clientID string) *common.ScreenshotDataPayload {
	return m.screenshotResults[clientID]
}

// MockClientMetadataUpdater implements ClientMetadataUpdater for testing
type MockClientMetadataUpdater struct {
	metadata map[string]*common.ClientMetadata
}

func NewMockClientMetadataUpdater() *MockClientMetadataUpdater {
	return &MockClientMetadataUpdater{
		metadata: make(map[string]*common.ClientMetadata),
	}
}

func (m *MockClientMetadataUpdater) UpdateClientMetadata(clientID string, fn func(*common.ClientMetadata)) {
	if _, exists := m.metadata[clientID]; !exists {
		m.metadata[clientID] = &common.ClientMetadata{ID: clientID}
	}
	fn(m.metadata[clientID])
}

func (m *MockClientMetadataUpdater) GetMetadata(clientID string) *common.ClientMetadata {
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

	if !d.HasHandler(common.MsgTypeCommandResult) {
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

	payload := common.CommandResultPayload{
		Success:  true,
		Output:   "test output",
		ExitCode: 0,
	}

	msg, _ := common.NewMessage(common.MsgTypeCommandResult, payload)
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

	payload := common.HeartbeatPayload{
		ClientID:   "client1",
		Status:     "online",
		CPUUsage:   45.5,
		MemUsage:   60.2,
		LastActive: time.Now(),
	}

	msg, _ := common.NewMessage(common.MsgTypeHeartbeat, payload)
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

	payload := common.FileListPayload{
		Path: "/test",
		Files: []common.FileInfo{
			{Name: "file1.txt", Path: "/test/file1.txt", Size: 100},
			{Name: "file2.txt", Path: "/test/file2.txt", Size: 200},
		},
	}

	msg, _ := common.NewMessage(common.MsgTypeFileList, payload)
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

	payload := common.ScreenshotDataPayload{
		Data:      []byte{0x1, 0x2, 0x3},
		Format:    "png",
		Width:     1024,
		Height:    768,
		Timestamp: time.Now(),
	}

	msg, _ := common.NewMessage(common.MsgTypeScreenshotData, payload)
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

	msg, _ := common.NewMessage(common.MsgTypeCommandResult, nil)
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

	if !d.HasHandler(common.MsgTypeCommandResult) {
		t.Fatal("HasHandler should return true for registered handler")
	}

	if d.HasHandler(common.MsgTypeFileList) {
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

	if d.HasHandler(common.MsgTypeCommandResult) &&
		d.HasHandler(common.MsgTypeFileList) &&
		d.HasHandler(common.MsgTypeHeartbeat) {
		// All handlers registered successfully
	} else {
		t.Fatal("All handlers should be registered")
	}
}
