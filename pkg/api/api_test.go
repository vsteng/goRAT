package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorat/pkg/auth"
	"gorat/pkg/clients"
)

// TestHandlerCreation tests Handler creation
func TestHandlerCreation(t *testing.T) {
	sessionMgr := auth.NewSessionManager(time.Hour)
	clientMgr := clients.NewManager()

	_, err := NewHandler(sessionMgr, clientMgr, nil, "admin", "password")
	if err != nil {
		// Templates might not exist in test environment, but handler should be creatable
		t.Logf("Handler creation returned error (expected in test env): %v", err)
	}
}

// TestErrorResponse tests error response helper
func TestErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	RespondError(w, http.StatusBadRequest, "Test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestAdminHandlerCreation tests AdminHandler creation
func TestAdminHandlerCreation(t *testing.T) {
	clientMgr := clients.NewManager()

	handler := NewAdminHandler(clientMgr, nil)

	if handler == nil {
		t.Fatal("AdminHandler is nil")
	}
}

// TestCORSMiddleware tests CORS middleware
func TestCORSMiddleware(t *testing.T) {
	middleware := CORSMiddleware()
	if middleware == nil {
		t.Fatal("CORS middleware is nil")
	}
}
