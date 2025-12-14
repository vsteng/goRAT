package server

import (
	"encoding/json"
	"net/http"
	"time"

	"gorat/pkg/logger"
	"gorat/pkg/protocol"
)

// HandleScreenshotRequest handles screenshot requests from web UI
func (wh *WebHandler) HandleScreenshotRequest(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Clear any previous result
	wh.server.ClearScreenshotResult(clientID)

	// Send screenshot request
	msg, err := protocol.NewMessage(protocol.MsgTypeTakeScreenshot, protocol.ScreenshotPayload{})
	if err != nil {
		logger.Get().ErrorWithErr("failed to create screenshot message", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(clientID, msg); err != nil {
		logger.Get().ErrorWithErr("failed to send screenshot request", err, "clientID", clientID)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	logger.Get().InfoWith("screenshot requested for client", "clientID", clientID)

	// Wait for response with timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			logger.Get().WarnWith("screenshot request timeout", "clientID", clientID)
			http.Error(w, "Request timeout", http.StatusRequestTimeout)
			return
		case <-ticker.C:
			if result := wh.server.GetScreenshotResult(clientID); result != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"width":  result.Width,
					"height": result.Height,
					"format": result.Format,
					"data":   result.Data,
				})
				wh.server.ClearScreenshotResult(clientID)
				return
			}
		}
	}
}
