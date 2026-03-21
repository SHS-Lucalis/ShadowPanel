package dlq

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	messages []FailedMessage
	mu       sync.RWMutex
	maxSize  int
}

func NewMemoryStore(maxSize int) *MemoryStore {
	if maxSize <= 0 {
		maxSize = 1000
	}

	return &MemoryStore{
		messages: make([]FailedMessage, 0),
		maxSize:  maxSize,
	}
}

func (s *MemoryStore) Push(_ context.Context, msg *FailedMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.messages) >= s.maxSize {
		s.messages = s.messages[1:]
	}

	s.messages = append(s.messages, *msg)

	return nil
}

func (s *MemoryStore) Pop(_ context.Context) (*FailedMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, msg := range s.messages {
		if !msg.Processed {
			s.messages = append(s.messages[:i], s.messages[i+1:]...)

			return &msg, nil
		}
	}

	return nil, ErrEmpty
}

func (s *MemoryStore) List(_ context.Context, limit, offset int) ([]FailedMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if offset >= len(s.messages) {
		return []FailedMessage{}, nil
	}

	end := min(offset+limit, len(s.messages))

	result := make([]FailedMessage, end-offset)
	copy(result, s.messages[offset:end])

	return result, nil
}

func (s *MemoryStore) Count(_ context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, msg := range s.messages {
		if !msg.Processed {
			count++
		}
	}

	return count, nil
}

func (s *MemoryStore) MarkProcessed(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.messages {
		if s.messages[i].ID == id {
			s.messages[i].Processed = true
			t := time.Now()
			s.messages[i].ProcessedAt = &t

			return nil
		}
	}

	return nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.messages {
		if s.messages[i].ID == id {
			s.messages = append(s.messages[:i], s.messages[i+1:]...)

			return nil
		}
	}

	return nil
}

func (s *MemoryStore) Purge(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = make([]FailedMessage, 0)

	return nil
}
