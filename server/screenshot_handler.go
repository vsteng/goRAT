package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"mww2.com/server_manager/common"
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
	msg, err := common.NewMessage(common.MsgTypeTakeScreenshot, common.ScreenshotPayload{})
	if err != nil {
		log.Printf("Failed to create screenshot message: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	if err := wh.clientMgr.SendToClient(clientID, msg); err != nil {
		log.Printf("Failed to send screenshot request to %s: %v", clientID, err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	log.Printf("Screenshot requested for client %s", clientID)

	// Wait for response with timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Printf("Screenshot request timeout for client %s", clientID)
			http.Error(w, "Request timeout", http.StatusRequestTimeout)
			return
		case <-ticker.C:
			if result, exists := wh.server.GetScreenshotResult(clientID); exists {
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
