package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const statusDispatchTimeout = 30 * time.Second

type statusDispatcher struct {
	ps         pubsub.PubSub
	gateway    StatusGateway
	registry   ConnectionChecker
	instanceID string
	logger     *slog.Logger

	mu          sync.RWMutex
	pendingReqs map[string]chan *messages.DaemonStatusResponsePayload
}

func NewStatusDispatcher(
	ps pubsub.PubSub,
	gateway StatusGateway,
	registry ConnectionChecker,
	instanceID string,
	logger *slog.Logger,
) StatusDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &statusDispatcher{
		ps:          ps,
		gateway:     gateway,
		registry:    registry,
		instanceID:  instanceID,
		logger:      logger,
		pendingReqs: make(map[string]chan *messages.DaemonStatusResponsePayload),
	}
}

func (d *statusDispatcher) Start(ctx context.Context) error {
	if err := d.ps.Subscribe(ctx, channels.DaemonStatusRequestAll, d.handleStatusRequest); err != nil {
		return errors.Wrap(err, "subscribe to status request dispatch")
	}

	responseChannel := channels.BuildDaemonStatusResponseChannel(d.instanceID)
	if err := d.ps.Subscribe(ctx, responseChannel, d.handleStatusResponse); err != nil {
		return errors.Wrap(err, "subscribe to status response")
	}

	d.logger.Info("status dispatcher started", "instance_id", d.instanceID)

	return nil
}

func (d *statusDispatcher) DispatchStatus(
	ctx context.Context, nodeID uint64,
) (*proto.StatusResponse, error) {
	resp, err := d.dispatchAndWait(ctx, nodeID, messages.DaemonStatusRequestPayload{
		NodeID:     nodeID,
		RequestID:  generateRequestID(),
		InstanceID: d.instanceID,
	})
	if err != nil {
		return nil, err
	}

	var statusResp proto.StatusResponse
	if err := statusResp.UnmarshalVT(resp.Data); err != nil {
		return nil, errors.Wrap(err, "unmarshal status response")
	}

	return &statusResp, nil
}

func (d *statusDispatcher) dispatchAndWait(
	ctx context.Context,
	nodeID uint64,
	payload messages.DaemonStatusRequestPayload,
) (*messages.DaemonStatusResponsePayload, error) {
	respCh := make(chan *messages.DaemonStatusResponsePayload, 1)

	d.mu.Lock()
	d.pendingReqs[payload.RequestID] = respCh
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pendingReqs, payload.RequestID)
		d.mu.Unlock()
	}()

	channel := channels.BuildDaemonStatusRequestChannel(nodeID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonStatusRequest, payload)
	if err != nil {
		return nil, errors.WithMessage(err, "create status request message")
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		return nil, errors.Wrap(err, "publish status request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(statusDispatchTimeout):
		return nil, errors.New("status dispatch timed out")
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("status dispatch request cancelled")
		}
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}

		return resp, nil
	}
}

func (d *statusDispatcher) handleStatusRequest(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonStatusRequestPayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse status request payload", "error", err)

		return nil
	}

	if !d.registry.IsConnected(payload.NodeID) {
		return nil
	}

	go d.executeAndRespond(payload) //nolint:gosec // intentionally outlives handler context

	return nil
}

func (d *statusDispatcher) executeAndRespond(payload messages.DaemonStatusRequestPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), statusDispatchTimeout)
	defer cancel()

	resp := d.executeStatusRequest(ctx, payload)
	resp.RequestID = payload.RequestID

	channel := channels.BuildDaemonStatusResponseChannel(payload.InstanceID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonStatusResponse, resp)
	if err != nil {
		d.logger.Error("failed to create status response message", "error", err)

		return
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		d.logger.Error("failed to publish status response",
			"request_id", payload.RequestID,
			"error", err,
		)
	}
}

func (d *statusDispatcher) executeStatusRequest(
	ctx context.Context,
	payload messages.DaemonStatusRequestPayload,
) messages.DaemonStatusResponsePayload {
	result, err := d.gateway.RequestStatus(ctx, payload.NodeID)
	if err != nil {
		return messages.DaemonStatusResponsePayload{Error: err.Error()}
	}

	data, err := result.MarshalVT()
	if err != nil {
		return messages.DaemonStatusResponsePayload{Error: err.Error()}
	}

	return messages.DaemonStatusResponsePayload{Data: data}
}

func (d *statusDispatcher) handleStatusResponse(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonStatusResponsePayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse status response payload", "error", err)

		return nil
	}

	d.mu.RLock()
	ch, ok := d.pendingReqs[payload.RequestID]
	d.mu.RUnlock()

	if ok {
		select {
		case ch <- &payload:
		default:
		}
	}

	return nil
}
