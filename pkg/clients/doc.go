// Package clients provides client lifecycle management and message routing.
//
// The clients package handles the registration, tracking, and communication
// with all connected WebSocket clients. It provides a clean interface-based
// API that abstracts away the complexity of channel-based event handling.
//
// The package is organized around two main interfaces:
//
// Client represents an individual connected client with methods for:
// - Accessing client ID, WebSocket connection, and metadata
// - Sending messages to the client
// - Updating client metadata
// - Checking connection status
//
// Manager manages all connected clients with methods for:
// - Registering and unregistering clients
// - Retrieving individual or all connected clients
// - Updating client metadata
// - Broadcasting messages to all clients
// - Controlling the lifecycle of the manager
//
// The Manager uses an internal event loop to handle concurrent operations
// safely. All operations are non-blocking and safe to use from multiple
// goroutines. The implementation uses:
//
// - Channels for event communication (register, unregister, broadcast)
// - RWMutex for thread-safe access to client storage
// - Goroutines for asynchronous message delivery
package clients
