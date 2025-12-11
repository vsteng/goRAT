package storage

import (
	"os"
	"testing"
	"time"

	"gorat/common"
)

func TestNewSQLiteStore(t *testing.T) {
	tmpFile := "test_storage.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("Store should not be nil")
	}
}

func TestSaveAndGetClient(t *testing.T) {
	tmpFile := "test_client.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	client := &common.ClientMetadata{
		ID:       "test-client-1",
		Hostname: "test-host",
		OS:       "Linux",
		Arch:     "x86_64",
		IP:       "192.168.1.100",
		PublicIP: "1.2.3.4",
		Alias:    "TestMachine",
		Status:   "online",
		Version:  "1.0.0",
		LastSeen: time.Now(),
	}

	err = store.SaveClient(client)
	if err != nil {
		t.Fatalf("Failed to save client: %v", err)
	}

	retrieved, err := store.GetClient("test-client-1")
	if err != nil {
		t.Fatalf("Failed to retrieve client: %v", err)
	}

	if retrieved.ID != "test-client-1" {
		t.Errorf("Expected ID 'test-client-1', got '%s'", retrieved.ID)
	}
	if retrieved.Hostname != "test-host" {
		t.Errorf("Expected hostname 'test-host', got '%s'", retrieved.Hostname)
	}
	if retrieved.Status != "online" {
		t.Errorf("Expected status 'online', got '%s'", retrieved.Status)
	}
}

func TestGetAllClients(t *testing.T) {
	tmpFile := "test_all_clients.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	for i := 1; i <= 3; i++ {
		client := &common.ClientMetadata{
			ID:       "client-" + string(rune(48+i)),
			Hostname: "host-" + string(rune(48+i)),
			OS:       "Linux",
			Status:   "online",
			LastSeen: time.Now(),
		}
		if err := store.SaveClient(client); err != nil {
			t.Fatalf("Failed to save client %d: %v", i, err)
		}
	}

	clients, err := store.GetAllClients()
	if err != nil {
		t.Fatalf("Failed to get all clients: %v", err)
	}

	if len(clients) != 3 {
		t.Errorf("Expected 3 clients, got %d", len(clients))
	}
}

func TestSaveAndGetProxy(t *testing.T) {
	tmpFile := "test_proxy.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	proxy := &ProxyConnection{
		ID:         "proxy-1",
		ClientID:   "client-1",
		LocalPort:  8080,
		RemoteHost: "example.com",
		RemotePort: 80,
		Protocol:   "tcp",
		CreatedAt:  time.Now(),
	}

	err = store.SaveProxy(proxy)
	if err != nil {
		t.Fatalf("Failed to save proxy: %v", err)
	}

	proxies, err := store.GetProxies("client-1")
	if err != nil {
		t.Fatalf("Failed to get proxies: %v", err)
	}

	if len(proxies) != 1 {
		t.Errorf("Expected 1 proxy, got %d", len(proxies))
	}

	if proxies[0].ID != "proxy-1" {
		t.Errorf("Expected proxy ID 'proxy-1', got '%s'", proxies[0].ID)
	}
}

func TestWebUserOperations(t *testing.T) {
	tmpFile := "test_users.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	err = store.CreateWebUser("testuser", "hashedpassword", "Test User", "admin")
	if err != nil {
		t.Fatalf("Failed to create web user: %v", err)
	}

	user, hash, err := store.GetWebUser("testuser")
	if err != nil {
		t.Fatalf("Failed to get web user: %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}

	if hash != "hashedpassword" {
		t.Errorf("Expected hash 'hashedpassword', got '%s'", hash)
	}

	exists, err := store.UserExists("testuser")
	if err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if !exists {
		t.Error("User should exist")
	}
}

func TestGetStats(t *testing.T) {
	tmpFile := "test_stats.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	clients := []*common.ClientMetadata{
		{ID: "online-1", Status: "online", LastSeen: time.Now()},
		{ID: "online-2", Status: "online", LastSeen: time.Now()},
		{ID: "offline-1", Status: "offline", LastSeen: time.Now()},
	}

	for _, client := range clients {
		if err := store.SaveClient(client); err != nil {
			t.Fatalf("Failed to save client: %v", err)
		}
	}

	total, online, offline, err := store.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}

	if online != 2 {
		t.Errorf("Expected online 2, got %d", online)
	}

	if offline != 1 {
		t.Errorf("Expected offline 1, got %d", offline)
	}
}

func TestServerSettings(t *testing.T) {
	tmpFile := "test_settings.db"
	defer os.Remove(tmpFile)

	store, err := NewSQLiteStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	err = store.SetServerSetting("key1", "value1")
	if err != nil {
		t.Fatalf("Failed to set server setting: %v", err)
	}

	value, err := store.GetServerSetting("key1")
	if err != nil {
		t.Fatalf("Failed to get server setting: %v", err)
	}

	if value != "value1" {
		t.Errorf("Expected value 'value1', got '%s'", value)
	}

	allSettings, err := store.GetAllServerSettings()
	if err != nil {
		t.Fatalf("Failed to get all settings: %v", err)
	}

	if len(allSettings) != 1 {
		t.Errorf("Expected 1 setting, got %d", len(allSettings))
	}
}
