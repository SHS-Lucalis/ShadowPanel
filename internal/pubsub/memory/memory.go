package memory

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gameap/gameap/internal/pubsub"
)

type Memory struct {
	handlers  map[string][]pubsub.Handler
	mu        sync.RWMutex
	logger    *slog.Logger
	closed    bool
	closeOnce sync.Once
}

func New() *Memory {
	return &Memory{
		handlers: make(map[string][]pubsub.Handler),
		logger:   slog.Default(),
	}
}

func (m *Memory) Publish(ctx context.Context, channel string, msg *pubsub.Message) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()

		return pubsub.ErrClosed
	}

	msg.Source = "memory"

	handlers := m.getMatchingHandlers(channel)
	m.mu.RUnlock()

	for _, handler := range handlers {
		pubsub.SafeCall(ctx, handler, msg, m.logger)
	}

	return nil
}

func (m *Memory) Subscribe(_ context.Context, pattern string, handler pubsub.Handler) error {
	if pattern == "" {
		return pubsub.ErrEmptyPattern
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return pubsub.ErrClosed
	}

	m.handlers[pattern] = append(m.handlers[pattern], handler)

	return nil
}

func (m *Memory) Unsubscribe(_ context.Context, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.handlers, pattern)

	return nil
}

func (m *Memory) Start(ctx context.Context) error {
	<-ctx.Done()

	return ctx.Err()
}

func (m *Memory) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		m.handlers = make(map[string][]pubsub.Handler)
		m.mu.Unlock()
	})

	return nil
}

func (m *Memory) getMatchingHandlers(channel string) []pubsub.Handler {
	var handlers []pubsub.Handler

	for pattern, h := range m.handlers {
		if pubsub.MatchPattern(pattern, channel) {
			handlers = append(handlers, h...)
		}
	}

	return handlers
}
