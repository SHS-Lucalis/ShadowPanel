package metrics

import (
	"sync"
	"time"

	"github.com/gameap/gameap/pkg/proto"
)

// ring is an in-memory FIFO buffer of MetricsResponse entries with a
// fixed capacity. Old entries are evicted when capacity is reached.
// Reads can additionally cap by age.
//
// Each nodeID has its own ring; this struct is the per-node container.
type ring struct {
	mu       sync.RWMutex
	capacity int
	entries  []*proto.MetricsResponse
}

func newRing(capacity int) *ring {
	if capacity < 1 {
		capacity = 1
	}

	return &ring{
		capacity: capacity,
		entries:  make([]*proto.MetricsResponse, 0, capacity),
	}
}

// Append adds a new entry, evicting the oldest if at capacity.
func (r *ring) Append(entry *proto.MetricsResponse) {
	if entry == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.entries) < r.capacity {
		r.entries = append(r.entries, entry)

		return
	}

	copy(r.entries, r.entries[1:])
	r.entries[len(r.entries)-1] = entry
}

// Snapshot returns entries newer than minTimestamp, ordered oldest →
// newest. Returns nil if buffer is empty or no entries match.
func (r *ring) Snapshot(minTimestamp time.Time) []*proto.MetricsResponse {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.entries) == 0 {
		return nil
	}

	idx := 0
	for idx < len(r.entries) {
		ts := r.entries[idx].GetTimestamp().AsTime()
		if !ts.Before(minTimestamp) {
			break
		}
		idx++
	}

	if idx >= len(r.entries) {
		return nil
	}

	out := make([]*proto.MetricsResponse, len(r.entries)-idx)
	copy(out, r.entries[idx:])

	return out
}

// Newest returns the most recently appended entry, or nil.
func (r *ring) Newest() *proto.MetricsResponse {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.entries) == 0 {
		return nil
	}

	return r.entries[len(r.entries)-1]
}

// Len returns the current number of entries.
func (r *ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.entries)
}
