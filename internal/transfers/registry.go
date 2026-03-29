package transfers

import (
	"context"
	"sync"
)

type Registry struct {
	mu     sync.Mutex
	states map[string]*State
}

func NewRegistry() *Registry {
	return &Registry{
		states: make(map[string]*State),
	}
}

func (r *Registry) Register(transferID string) *State {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := &State{
		ch: make(chan struct{}),
	}
	r.states[transferID] = state

	return state
}

func (r *Registry) Get(transferID string) (*State, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[transferID]

	return state, ok
}

func (r *Registry) Unregister(transferID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.states, transferID)
}

type State struct {
	mu       sync.Mutex
	parts    int
	complete bool
	err      error
	ch       chan struct{}
}

func (s *State) AddPart() {
	s.mu.Lock()
	s.parts++
	ch := s.ch
	s.ch = make(chan struct{})
	s.mu.Unlock()
	close(ch)
}

func (s *State) Complete() {
	s.mu.Lock()
	s.complete = true
	ch := s.ch
	s.ch = make(chan struct{})
	s.mu.Unlock()
	close(ch)
}

func (s *State) SetError(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	ch := s.ch
	s.ch = make(chan struct{})
	s.mu.Unlock()
	close(ch)
}

// WaitForPart blocks until the given part number is available, the transfer
// completes, an error occurs, or the context is cancelled.
// Check order: available parts first (serve valid data before reporting errors),
// then error, then completion.
func (s *State) WaitForPart(ctx context.Context, partNum int) (bool, error) {
	for {
		s.mu.Lock()
		if s.parts > partNum {
			s.mu.Unlock()

			return true, nil
		}
		if s.err != nil {
			err := s.err
			s.mu.Unlock()

			return false, err
		}
		if s.complete {
			s.mu.Unlock()

			return false, nil
		}
		ch := s.ch
		s.mu.Unlock()

		select {
		case <-ch:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}
