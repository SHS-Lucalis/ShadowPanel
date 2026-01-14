package pubsub

import (
	"context"
	"time"
)

// Message represents a pub-sub message.
type Message struct {
	ID        string    `json:"id"`
	Channel   string    `json:"channel"`
	Type      string    `json:"type"`
	Payload   []byte    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source,omitempty"`
}

// Handler is a function that processes incoming messages.
// It should return quickly; long-running operations should be dispatched to goroutines.
type Handler func(ctx context.Context, msg *Message) error

// Publisher defines the interface for publishing messages.
type Publisher interface {
	Publish(ctx context.Context, channel string, msg *Message) error
}

// Subscriber defines the interface for subscribing to channels.
type Subscriber interface {
	// Subscribe registers a handler for the specified channel pattern.
	// The pattern can include trailing wildcard (e.g., "cache:*", "events:server:*").
	Subscribe(ctx context.Context, pattern string, handler Handler) error

	Unsubscribe(ctx context.Context, pattern string) error
}

// PubSub combines Publisher and Subscriber interfaces.
type PubSub interface {
	Publisher
	Subscriber

	// Start begins processing subscriptions.
	// This method blocks until the context is cancelled.
	Start(ctx context.Context) error

	Close() error
}
