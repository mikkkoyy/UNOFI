package events

import (
	"sync"
)

// Type represents the type of event.
type Type string

const (
	EventDeviceConnected    Type = "device.connected"
	EventDeviceDisconnected Type = "device.disconnected"
	EventSessionCreated     Type = "session.created"
	EventSessionExpired     Type = "session.expired"
	EventPaymentReceived    Type = "payment.received"
	EventVoucherRedeemed    Type = "voucher.redeemed"
	EventClientAuthorized   Type = "client.authorized"
	EventClientDeauthorized Type = "client.deauthorized"
)

// Event represents a system event.
type Event struct {
	Type Type        `json:"type"`
	Data interface{} `json:"data"`
}

// Handler is a function that handles events.
type Handler func(event Event)

// Bus provides a simple pub/sub event system.
type Bus struct {
	mu       sync.RWMutex
	handlers map[Type][]Handler
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[Type][]Handler),
	}
}

// Subscribe registers a handler for an event type.
func (b *Bus) Subscribe(eventType Type, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish sends an event to all registered handlers.
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}
}
