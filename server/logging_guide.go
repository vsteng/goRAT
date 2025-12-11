package server

import (
	"log"
)

// IntegrateLogging demonstrates how to use structured logging throughout the server.
// This file shows best practices for structured logging integration.

/*
STRUCTURED LOGGING INTEGRATION GUIDE

The server now uses slog-based structured logging initialized in main.go:

1. Initialization in main.go:
   ```go
   logger.Init(logger.LogLevel(*logLevel), *logFormat)
   log := logger.Get()
   ```

2. Usage patterns:

   // Info message with context
   log.InfoWith("client_connected", "client_id", clientID, "ip", ip)

   // Warning with attributes
   log.WarnWith("slow_operation", "duration_ms", duration)

   // Error with error object
   log.ErrorWithErr("database_error", err, "table", "clients")

   // Debug for development
   log.DebugWith("protocol_message", "type", msgType, "length", len(data))

3. Configuration (from config.yaml or environment):
   - LOG_LEVEL: debug, info, warn, error (default: info)
   - LOG_FORMAT: text or json (default: text)

4. Output formats:

   Text format (human-readable):
   2025-12-11T16:10:23.456Z	INFO	server	client_connected	client_id=abc123	ip=192.168.1.100

   JSON format (machine-parseable):
   {"time":"2025-12-11T16:10:23.456Z","level":"INFO","msg":"client_connected","client_id":"abc123","ip":"192.168.1.100"}

5. Migration checklist for existing code:
   - Replace log.Printf() → log.InfoWith(msg, key, value)
   - Replace log.Println(err) → log.ErrorWithErr(msg, err)
   - Replace log.Fatal() → log.ErrorWithErr(msg, err); return
   - Add contextual attributes (client_id, ip, user, request_id, etc)

*/

// Integration examples for key components:

// ClientManager logging example
/*
In client_manager.go Run() method:
	log := logger.Get()
	log.InfoWith("client_manager_started")
	for {
		select {
		case client := <-m.register:
			log.InfoWith("client_registered", "client_id", client.ID)
			m.handleRegister(client)
		case client := <-m.unregister:
			log.InfoWith("client_unregistered", "client_id", client.ID)
			m.handleUnregister(client)
		}
	}
*/

// ClientStore logging example
/*
In client_store.go SaveClient() method:
	log := logger.Get()
	log.InfoWith("saving_client", "client_id", client.ID, "hostname", client.Metadata.Hostname)
	if err := s.db.Exec(...); err != nil {
		log.ErrorWithErr("failed_to_save_client", err, "client_id", client.ID)
		return err
	}
	log.DebugWith("client_saved", "client_id", client.ID)
*/

// WebSocket handler logging example
/*
In handlers.go HandleWebSocket() method:
	log := logger.Get().With("client_id", clientID)
	log.InfoWith("websocket_connected")
	for {
		var msg common.Message
		if err := conn.ReadJSON(&msg); err != nil {
			log.ErrorWithErr("failed_to_read_message", err)
			return
		}
		log.DebugWith("message_received", "type", msg.Type)
	}
*/

// DeprecationNotice marks the old log package usage for migration
func DeprecationNotice() {
	// This demonstrates what should be replaced
	log.Println("DEPRECATED: Use logger.Get().InfoWith() instead")
	log.Printf("DEPRECATED: Use logger.Get().ErrorWithErr() instead")
}
