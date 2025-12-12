package messaging

import (
"fmt"
"log"
"sync"

"gorat/pkg/protocol"
)

// DispatcherImpl implements the Dispatcher interface
type DispatcherImpl struct {
	handlers map[protocol.MessageType]Handler
	mu       sync.RWMutex
}

// NewDispatcher creates a new message dispatcher
func NewDispatcher() *DispatcherImpl {
	return &DispatcherImpl{
		handlers: make(map[protocol.MessageType]Handler),
	}
}

// Register registers a handler for a message type
func (d *DispatcherImpl) Register(handler Handler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	msgType := handler.MessageType()
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.handlers[msgType]; exists {
		return fmt.Errorf("handler already registered for message type: %s", msgType)
	}

	d.handlers[msgType] = handler
	log.Printf("Registered handler for message type: %s", msgType)
	return nil
}

// Dispatch dispatches a message to the appropriate handler
func (d *DispatcherImpl) Dispatch(clientID string, msg *protocol.Message) (interface{}, error) {
	d.mu.RLock()
	handler, exists := d.handlers[msg.Type]
	d.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for message type: %s", msg.Type)
	}

	return handler.Handle(clientID, msg)
}

// HasHandler checks if a handler exists for the message type
func (d *DispatcherImpl) HasHandler(msgType protocol.MessageType) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.handlers[msgType]
	return exists
}
