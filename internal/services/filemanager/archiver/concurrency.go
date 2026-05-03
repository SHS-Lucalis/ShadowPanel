package archiver

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

var ErrTooManyConcurrent = errors.New("too many concurrent archive downloads for this server")

type InMemoryConcurrencyGuard struct {
	mu     sync.Mutex
	counts map[uint]uint32
	limit  uint32
}

func NewInMemoryConcurrencyGuard(limit uint32) *InMemoryConcurrencyGuard {
	if limit == 0 {
		limit = 1
	}

	return &InMemoryConcurrencyGuard{
		counts: make(map[uint]uint32),
		limit:  limit,
	}
}

func (g *InMemoryConcurrencyGuard) Acquire(_ context.Context, serverID uint) (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.counts[serverID] >= g.limit {
		return nil, ErrTooManyConcurrent
	}

	g.counts[serverID]++

	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()

		if g.counts[serverID] == 0 {
			return
		}
		g.counts[serverID]--
		if g.counts[serverID] == 0 {
			delete(g.counts, serverID)
		}
	}, nil
}
