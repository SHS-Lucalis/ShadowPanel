package metrics

import (
	"sync"

	"github.com/gameap/gameap/pkg/proto"
)

type subscription struct {
	hub    *hub
	nodeID uint64

	samplesCh chan *proto.MetricsResponse

	closeMu sync.Mutex
	closed  bool
}

func newSubscription(h *hub, nodeID uint64, bufferSize int) *subscription {
	return &subscription{
		hub:       h,
		nodeID:    nodeID,
		samplesCh: make(chan *proto.MetricsResponse, bufferSize),
	}
}

func (s *subscription) Samples() <-chan *proto.MetricsResponse {
	return s.samplesCh
}

func (s *subscription) Close() {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()

		return
	}
	s.closed = true
	s.closeMu.Unlock()

	s.hub.unsubscribe(s)
}

// deliver attempts a non-blocking send. Drops on full buffer to keep
// the producer (pubsub fan-out) non-blocking; consumers are expected
// to drain promptly.
func (s *subscription) deliver(entry *proto.MetricsResponse) {
	s.closeMu.Lock()
	closed := s.closed
	s.closeMu.Unlock()

	if closed {
		return
	}

	select {
	case s.samplesCh <- entry:
	default:
	}
}

func (s *subscription) closeChannel() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if !s.closed {
		s.closed = true
	}

	close(s.samplesCh)
}
