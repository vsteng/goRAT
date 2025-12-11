package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorat/pkg/auth"
	"gorat/pkg/clients"
) // TestNewTerminalProxy verifies TerminalProxy creation
func TestNewTerminalProxy(t *testing.T) {
	clientMgr := clients.NewManager()
	sessionMgr := auth.NewSessionManager(1 * time.Hour)

	tp := NewTerminalProxy(clientMgr, sessionMgr)

	if tp == nil {
		t.Fatal("TerminalProxy is nil")
	}
	if tp.clientMgr == nil {
		t.Error("clientMgr not set")
	}
	if tp.sessionMgr == nil {
		t.Error("sessionMgr not set")
	}
	if tp.sessions == nil {
		t.Error("sessions map not initialized")
	}
}

// TestTerminalProxySessionCreation verifies session creation
func TestTerminalProxySessionCreation(t *testing.T) {
	session := &TerminalProxySession{
		ID:       "session-123",
		ClientID: "client-456",
		WebConn:  nil,
	}

	if session.ID != "session-123" {
		t.Error("Session ID not set correctly")
	}
	if session.ClientID != "client-456" {
		t.Error("Session ClientID not set correctly")
	}
}

// TestHandleTerminalWebSocketNoAuth tests unauthorized access
func TestHandleTerminalWebSocketNoAuth(t *testing.T) {
	clientMgr := clients.NewManager()
	sessionMgr := auth.NewSessionManager(1 * time.Hour)
	tp := NewTerminalProxy(clientMgr, sessionMgr)

	req := httptest.NewRequest("GET", "/ws/terminal", nil)
	w := httptest.NewRecorder()

	tp.HandleTerminalWebSocket(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestTerminalProxySessionMutex verifies thread-safe session operations
func TestTerminalProxySessionMutex(t *testing.T) {
	session := &TerminalProxySession{
		ID:       "test-session",
		ClientID: "test-client",
	}

	// Should not panic when locking/unlocking
	session.mu.Lock()
	defer session.mu.Unlock()

	session.ID = "modified-session"
	if session.ID != "modified-session" {
		t.Error("Session ID not modified correctly")
	}
}

// TestTerminalProxyThreadSafety verifies RWMutex operations
func TestTerminalProxyThreadSafety(t *testing.T) {
	clientMgr := clients.NewManager()
	sessionMgr := auth.NewSessionManager(1 * time.Hour)
	tp := NewTerminalProxy(clientMgr, sessionMgr)

	// Should not panic when locking/unlocking
	tp.mu.Lock()
	tp.sessions["test-id"] = &TerminalProxySession{ID: "test-id"}
	tp.mu.Unlock()

	tp.mu.RLock()
	defer tp.mu.RUnlock()

	if _, exists := tp.sessions["test-id"]; !exists {
		t.Error("Session not found after storage")
	}
}
