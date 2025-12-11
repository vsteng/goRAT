package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"gorat/common"
	"gorat/pkg/auth"

	"github.com/gorilla/websocket"
)

// TerminalProxy manages terminal WebSocket connections between web UI and clients
type TerminalProxy struct {
	clientMgr  *ClientManager
	sessions   map[string]*TerminalProxySession
	mu         sync.RWMutex
	sessionMgr auth.SessionManager
}

// TerminalProxySession represents a terminal proxy session
type TerminalProxySession struct {
	ID       string
	ClientID string
	WebConn  *websocket.Conn
	mu       sync.Mutex
}

// NewTerminalProxy creates a new terminal proxy
func NewTerminalProxy(clientMgr *ClientManager, sessionMgr auth.SessionManager) *TerminalProxy {
	return &TerminalProxy{
		clientMgr:  clientMgr,
		sessions:   make(map[string]*TerminalProxySession),
		sessionMgr: sessionMgr,
	}
}

// HandleTerminalWebSocket handles terminal WebSocket connections from web UI
func (tp *TerminalProxy) HandleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	cookie, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if _, exists := tp.sessionMgr.GetSession(cookie.Value); !exists {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get client ID from query
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Check if client is connected
	client, exists := tp.clientMgr.GetClient(clientID)
	if !exists || client == nil {
		http.Error(w, "Client not found or offline", http.StatusNotFound)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// Generate session ID
	sessionID := common.GenerateID()

	// Create proxy session
	session := &TerminalProxySession{
		ID:       sessionID,
		ClientID: clientID,
		WebConn:  conn,
	}

	tp.mu.Lock()
	tp.sessions[sessionID] = session
	tp.mu.Unlock()

	defer func() {
		tp.mu.Lock()
		delete(tp.sessions, sessionID)
		tp.mu.Unlock()
		conn.Close()

		// Send stop terminal message to client
		tp.stopTerminalOnClient(clientID, sessionID)
	}()

	// Start terminal on client
	if err := tp.startTerminalOnClient(clientID, sessionID); err != nil {
		log.Printf("Failed to start terminal on client: %v", err)
		tp.sendWebError(conn, "Failed to start terminal session")
		return
	}

	// Handle messages from web UI
	go tp.handleWebMessages(session)

	// Keep connection alive
	select {}
}

// startTerminalOnClient sends a start terminal message to the client
func (tp *TerminalProxy) startTerminalOnClient(clientID, sessionID string) error {
	payload := &common.StartTerminalPayload{
		SessionID: sessionID,
		Rows:      24,
		Cols:      80,
	}

	msg, err := common.NewMessage(common.MsgTypeStartTerminal, payload)
	if err != nil {
		return err
	}

	return tp.clientMgr.SendToClient(clientID, msg)
}

// stopTerminalOnClient sends a stop terminal message to the client
func (tp *TerminalProxy) stopTerminalOnClient(clientID, sessionID string) {
	payload := &common.TerminalInputPayload{
		SessionID: sessionID,
	}

	msg, err := common.NewMessage(common.MsgTypeStopTerminal, payload)
	if err != nil {
		return
	}

	tp.clientMgr.SendToClient(clientID, msg)
}

// handleWebMessages handles messages from the web UI
func (tp *TerminalProxy) handleWebMessages(session *TerminalProxySession) {
	for {
		_, message, err := session.WebConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse message
		var webMsg struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}

		if err := json.Unmarshal(message, &webMsg); err != nil {
			log.Printf("Failed to parse web message: %v", err)
			continue
		}

		// Handle different message types
		switch webMsg.Type {
		case "input":
			// Forward input to client
			tp.forwardInputToClient(session.ClientID, session.ID, webMsg.Data)
		case "interrupt":
			// Send Ctrl+C
			tp.forwardInputToClient(session.ClientID, session.ID, "\x03")
		case "resize":
			// Handle terminal resize (future enhancement)
		}
	}
}

// forwardInputToClient forwards input from web UI to client
func (tp *TerminalProxy) forwardInputToClient(clientID, sessionID, data string) {
	payload := &common.TerminalInputPayload{
		SessionID: sessionID,
		Data:      data,
	}

	msg, err := common.NewMessage(common.MsgTypeTerminalInput, payload)
	if err != nil {
		log.Printf("Failed to create terminal input message: %v", err)
		return
	}

	if err := tp.clientMgr.SendToClient(clientID, msg); err != nil {
		log.Printf("Failed to send terminal input to client: %v", err)
	}
}

// HandleTerminalOutput handles terminal output from client
func (tp *TerminalProxy) HandleTerminalOutput(sessionID, data string, isError bool) {
	tp.mu.RLock()
	session, exists := tp.sessions[sessionID]
	tp.mu.RUnlock()

	if !exists {
		return
	}

	msgType := "output"
	if isError {
		msgType = "error"
	}

	response := map[string]string{
		"type": msgType,
		"data": data,
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if err := session.WebConn.WriteJSON(response); err != nil {
		log.Printf("Failed to send output to web UI: %v", err)
	}
}

// sendWebError sends an error message to the web UI
func (tp *TerminalProxy) sendWebError(conn *websocket.Conn, message string) {
	response := map[string]string{
		"type": "error",
		"data": message,
	}
	conn.WriteJSON(response)
}

// GetSessionClientID returns the client ID for a session
func (tp *TerminalProxy) GetSessionClientID(sessionID string) (string, bool) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	session, exists := tp.sessions[sessionID]
	if !exists {
		return "", false
	}

	return session.ClientID, true
}
