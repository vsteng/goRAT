package auth

import (
"testing"
"time"

"gorat/common"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager(1 * time.Hour)
	if sm == nil {
		t.Fatal("SessionManager should not be nil")
	}
}

func TestCreateSession(t *testing.T) {
	sm := NewSessionManager(1 * time.Hour)
	session, err := sm.CreateSession("testuser")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if session == nil {
		t.Fatal("Session should not be nil")
	}
	if session.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", session.Username)
	}
	if session.ID == "" {
		t.Fatal("Session ID should not be empty")
	}
}

func TestGetSession(t *testing.T) {
	sm := NewSessionManager(1 * time.Hour)
	session, err := sm.CreateSession("testuser")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	retrieved, exists := sm.GetSession(session.ID)
	if !exists {
		t.Fatal("Session should exist")
	}
	if retrieved.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", retrieved.Username)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	sm := NewSessionManager(1 * time.Hour)
	_, exists := sm.GetSession("nonexistent")
	if exists {
		t.Fatal("Session should not exist")
	}
}

func TestSessionExpiration(t *testing.T) {
	sm := NewSessionManager(1 * time.Millisecond)
	session, err := sm.CreateSession("testuser")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	_, exists := sm.GetSession(session.ID)
	if exists {
		t.Fatal("Expired session should not exist")
	}
}

func TestDeleteSession(t *testing.T) {
	sm := NewSessionManager(1 * time.Hour)
	session, err := sm.CreateSession("testuser")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sm.DeleteSession(session.ID)
	_, exists := sm.GetSession(session.ID)
	if exists {
		t.Fatal("Deleted session should not exist")
	}
}

func TestGetAllSessions(t *testing.T) {
	sm := NewSessionManager(1 * time.Hour)
	for i := 0; i < 3; i++ {
		_, err := sm.CreateSession("user" + string(rune('1'+i)))
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}
	sessions := sm.GetAllSessions()
	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}
}

func TestNewAuthenticator(t *testing.T) {
	auth := NewAuthenticator("test-token")
	if auth == nil {
		t.Fatal("Authenticator should not be nil")
	}
}

func TestAuthenticateSuccess(t *testing.T) {
	auth := NewAuthenticator("test-token")
	payload := &common.AuthPayload{
		ClientID: "test-client",
		Token:    "test-token",
	}
	success, token := auth.Authenticate(payload)
	if !success {
		t.Fatal("Authentication should succeed")
	}
	if token == "" {
		t.Fatal("Token should not be empty")
	}
}

func TestAuthenticateFail(t *testing.T) {
	auth := NewAuthenticator("test-token")
	payload := &common.AuthPayload{
		ClientID: "test-client",
		Token:    "wrong-token",
	}
	success, token := auth.Authenticate(payload)
	if success {
		t.Fatal("Authentication should fail")
	}
	if token != "" {
		t.Fatal("Token should be empty on auth failure")
	}
}

func TestSessionIsExpired(t *testing.T) {
	now := time.Now()
	session1 := &Session{
		ID:        "1",
		Username:  "user",
		CreatedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if session1.IsExpired() {
		t.Fatal("Non-expired session should not be expired")
	}

	session2 := &Session{
		ID:        "2",
		Username:  "user",
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	if !session2.IsExpired() {
		t.Fatal("Expired session should be expired")
	}
}
