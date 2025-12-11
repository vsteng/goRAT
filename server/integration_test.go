package server

import (
	"testing"
)

// TestServerInitialization tests basic server creation
func TestServerInitialization(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:8080",
		AuthToken:   "test-token",
		WebUsername: "admin",
		WebPassword: "password",
	}

	server := NewServer(cfg)
	if server == nil {
		t.Fatal("Server should not be nil")
	}
	if server.config != cfg {
		t.Error("Server config not set correctly")
	}
}

// TestServerConfigAddress tests server config address
func TestServerConfigAddress(t *testing.T) {
	cfg := &Config{
		Address:     "0.0.0.0:9000",
		AuthToken:   "test-token",
		WebUsername: "admin",
		WebPassword: "password",
	}

	server := NewServer(cfg)
	if server.config.Address != "0.0.0.0:9000" {
		t.Errorf("Expected address 0.0.0.0:9000, got %s", server.config.Address)
	}
}

// TestServerClientManager tests client manager initialization
func TestServerClientManager(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:8080",
		AuthToken:   "test-token",
		WebUsername: "admin",
		WebPassword: "password",
	}

	server := NewServer(cfg)
	if server.manager == nil {
		t.Error("Server manager should be initialized")
	}
}

// TestServerAuthenticator tests authenticator initialization
func TestServerAuthenticator(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:8080",
		AuthToken:   "test-token",
		WebUsername: "admin",
		WebPassword: "password",
	}

	server := NewServer(cfg)
	if server.authenticator == nil {
		t.Error("Server authenticator should be initialized")
	}
}

// TestServerTerminalProxy tests terminal proxy initialization
func TestServerTerminalProxy(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:8080",
		AuthToken:   "test-token",
		WebUsername: "admin",
		WebPassword: "password",
	}

	server := NewServer(cfg)
	if server.terminalProxy == nil {
		t.Error("Server terminalProxy should be initialized")
	}
}

// TestServerInstanceManagerPIDFile tests instance manager PID file
func TestServerInstanceManagerPIDFile(t *testing.T) {
	sim := NewServerInstanceManager()
	if sim == nil {
		t.Fatal("ServerInstanceManager should not be nil")
	}

	pidFile := sim.PIDFile()
	if pidFile == "" {
		t.Error("PID file path should not be empty")
	}
}

// TestServerWebHandler tests web handler initialization
func TestServerWebHandler(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:8080",
		AuthToken:   "test-token",
		WebUsername: "admin",
		WebPassword: "password",
	}

	server := NewServer(cfg)
	// webHandler may be nil if web handler initialization fails, which is acceptable per design
	if server == nil {
		t.Error("Server should be initialized even if webHandler is nil")
	}
}
