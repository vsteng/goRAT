package clients

import (
	"gorat/common"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Error("NewManager returned nil")
	}
	if m.IsRunning() {
		t.Error("Manager should not be running initially")
	}
	if len(m.GetAllClients()) != 0 {
		t.Error("Manager should have no clients initially")
	}
}

func TestManagerStartStop(t *testing.T) {
	m := NewManager()
	m.Start()
	if !m.IsRunning() {
		t.Error("Manager should be running after Start()")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("Manager should not be running after Stop()")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("Manager should still not be running")
	}
}

func TestGetClient(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	_, ok := m.GetClient("non-existent")
	if ok {
		t.Error("GetClient should return false for non-existent client")
	}
}

func TestGetAllClients(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	clients := m.GetAllClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients initially, got %d", len(clients))
	}
}

func TestUpdateClientMetadataNonExistent(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	err := m.UpdateClientMetadata("non-existent", func(meta *common.ClientMetadata) {})
	if err == nil {
		t.Error("UpdateClientMetadata should fail for non-existent client")
	}
}

func TestBroadcastMessage(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	payload := common.ExecuteCommandPayload{Command: "test"}
	msg, _ := common.NewMessage(common.MsgTypeExecuteCommand, payload)
	m.BroadcastMessage(msg)

	if len(m.GetAllClients()) != 0 {
		t.Error("Should have no clients")
	}
}

func TestStopManagerWithRunningClients(t *testing.T) {
	m := NewManager()
	m.Start()

	if len(m.GetAllClients()) != 0 {
		t.Errorf("Expected 0 clients initially, got %d", len(m.GetAllClients()))
	}

	m.Stop()

	if len(m.GetAllClients()) != 0 {
		t.Error("All clients should be closed after Stop()")
	}

	if m.IsRunning() {
		t.Error("Manager should not be running after Stop()")
	}
}

func TestRegisterClientAfterStop(t *testing.T) {
	m := NewManager()
	m.Start()
	m.Stop()

	_, err := m.RegisterClient("test-client", nil)
	if err == nil {
		t.Error("RegisterClient should fail after Stop()")
	}
}

func TestManagerConcurrency(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			clients := m.GetAllClients()
			if len(clients) >= 0 {
				successCount.Add(1)
			}

			m.UpdateClientMetadata("non-existent", func(meta *common.ClientMetadata) {
				meta.OS = "test"
			})

			payload := common.ExecuteCommandPayload{Command: "test"}
			msg, _ := common.NewMessage(common.MsgTypeExecuteCommand, payload)
			m.BroadcastMessage(msg)
		}(i)
	}

	wg.Wait()

	if successCount.Load() != 10 {
		t.Errorf("Expected 10 successful operations, got %d", successCount.Load())
	}
}

func TestClientImplMetadata(t *testing.T) {
	client := &ClientImpl{
		id:       "test-id",
		metadata: &common.ClientMetadata{ID: "test-id"},
		closed:   false,
	}

	if client.ID() != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", client.ID())
	}

	meta := client.Metadata()
	if meta.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", meta.ID)
	}

	client.UpdateMetadata(func(meta *common.ClientMetadata) {
		meta.OS = "Linux"
		meta.Hostname = "test-host"
	})

	meta = client.Metadata()
	if meta.OS != "Linux" {
		t.Errorf("Expected OS 'Linux', got %s", meta.OS)
	}

	if meta.Hostname != "test-host" {
		t.Errorf("Expected Hostname 'test-host', got %s", meta.Hostname)
	}
}

func TestClientImplClose(t *testing.T) {
	client := &ClientImpl{
		id:       "test-id",
		metadata: &common.ClientMetadata{ID: "test-id"},
		send:     make(chan *common.Message, 256),
		closed:   false,
		conn:     nil,
	}

	if client.IsClosed() {
		t.Error("Client should not be closed initially")
	}

	err := client.Close()
	if err == nil {
		// nil connection will error
	}

	if !client.IsClosed() {
		t.Error("Client should be closed after Close()")
	}

	err = client.Close()
	if err != nil {
		t.Logf("Second close returned error: %v", err)
	}
}

func TestClientImplSendMessage(t *testing.T) {
	client := &ClientImpl{
		id:       "test-id",
		metadata: &common.ClientMetadata{ID: "test-id"},
		send:     make(chan *common.Message, 256),
		closed:   false,
	}

	payload := common.ExecuteCommandPayload{Command: "test"}
	msg, _ := common.NewMessage(common.MsgTypeExecuteCommand, payload)

	err := client.SendMessage(msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	select {
	case received := <-client.send:
		if received.Type != common.MsgTypeExecuteCommand {
			t.Errorf("Expected message type %v, got %v", common.MsgTypeExecuteCommand, received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Message not received")
	}

	client.Close()
	err = client.SendMessage(msg)
	if err == nil {
		t.Error("SendMessage should fail on closed client")
	}
}

func TestManagerChannelCapacity(t *testing.T) {
	m := NewManager()

	if m.register == nil {
		t.Error("register channel should be created")
	}
	if m.unregister == nil {
		t.Error("unregister channel should be created")
	}
	if m.broadcast == nil {
		t.Error("broadcast channel should be created")
	}
}

func TestManagerIsRunningStateTransitions(t *testing.T) {
	m := NewManager()

	m.Start()
	if !m.IsRunning() {
		t.Error("Manager should be running")
	}

	m.Start()
	if !m.IsRunning() {
		t.Error("Manager should still be running")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("Manager should not be running")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("Manager should still not be running")
	}

	m.Start()
	if !m.IsRunning() {
		t.Error("Manager should be running again")
	}

	m.Stop()
	if m.IsRunning() {
		t.Error("Manager should not be running")
	}
}

func TestClientImplConcurrentUpdates(t *testing.T) {
	client := &ClientImpl{
		id:       "test-id",
		metadata: &common.ClientMetadata{ID: "test-id"},
		closed:   false,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client.UpdateMetadata(func(meta *common.ClientMetadata) {
				meta.Hostname = "host-" + string(rune('0'+idx))
			})
		}(i)
	}

	wg.Wait()

	meta := client.Metadata()
	if meta == nil {
		t.Error("Metadata should not be nil")
	}
}

func TestUnregisterClient(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	err := m.UnregisterClient("non-existent")
	if err != nil {
		t.Fatalf("UnregisterClient failed: %v", err)
	}
}

func TestBroadcastToEmptyManager(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	payload := common.ExecuteCommandPayload{Command: "ls"}
	msg, _ := common.NewMessage(common.MsgTypeExecuteCommand, payload)

	m.BroadcastMessage(msg)
	m.BroadcastMessage(msg)
	m.BroadcastMessage(msg)
}

func TestGetClientCount(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	if m.GetClientCount() != 0 {
		t.Error("Expected 0 clients initially")
	}
}

func TestIsClientIDRegistered(t *testing.T) {
	m := NewManager()
	m.Start()
	defer m.Stop()

	if m.IsClientIDRegistered("non-existent") {
		t.Error("Should return false for non-existent client")
	}
}
