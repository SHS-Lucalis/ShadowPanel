package session

import (
	"context"
	"io"
	"slices"
	"sync"

	"github.com/gameap/gameap/pkg/proto"
)

// stubStream is a test double for the Stream interface that records sent
// gateway messages and serves Recv from a controllable channel.
type stubStream struct {
	ctx     context.Context
	sentMu  sync.Mutex
	sent    []*proto.GatewayMessage
	sendErr error
	recvCh  chan *proto.DaemonMessage
}

func newStubStream(ctx context.Context) *stubStream {
	return &stubStream{
		ctx:    ctx,
		recvCh: make(chan *proto.DaemonMessage, 16),
	}
}

func (s *stubStream) Send(m *proto.GatewayMessage) error {
	if s.sendErr != nil {
		return s.sendErr
	}

	s.sentMu.Lock()
	defer s.sentMu.Unlock()
	s.sent = append(s.sent, m)

	return nil
}

func (s *stubStream) Recv() (*proto.DaemonMessage, error) {
	msg, ok := <-s.recvCh
	if !ok {
		return nil, io.EOF
	}

	return msg, nil
}

func (s *stubStream) Context() context.Context {
	return s.ctx
}

// Sent returns a snapshot of all messages that have been Send-called on the
// stub stream so far.
func (s *stubStream) Sent() []*proto.GatewayMessage {
	s.sentMu.Lock()
	defer s.sentMu.Unlock()

	return slices.Clone(s.sent)
}

// fakeMetricsWaiterRegistrar records calls to RegisterRemoteWaiter and
// CancelWaiter for assertion purposes.
type fakeMetricsWaiterRegistrar struct {
	mu         sync.Mutex
	registered []registeredWaiter
	canceled   []string
}

type registeredWaiter struct {
	requestID  string
	nodeID     uint64
	instanceID string
}

func (f *fakeMetricsWaiterRegistrar) RegisterRemoteWaiter(requestID string, nodeID uint64, instanceID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered = append(f.registered, registeredWaiter{
		requestID:  requestID,
		nodeID:     nodeID,
		instanceID: instanceID,
	})
}

func (f *fakeMetricsWaiterRegistrar) CancelWaiter(requestID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.canceled = append(f.canceled, requestID)
}

func (f *fakeMetricsWaiterRegistrar) Registered() []registeredWaiter {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.registered)
}

func (f *fakeMetricsWaiterRegistrar) Canceled() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.canceled)
}
