package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const consoleLogDispatchTimeout = 30 * time.Second

type consoleLogDispatcher struct {
	ps         pubsub.PubSub
	gateway    ConsoleLogGateway
	registry   ConnectionChecker
	instanceID string
	logger     *slog.Logger

	mu          sync.RWMutex
	pendingReqs map[string]chan *messages.DaemonConsoleLogResponsePayload
}

func NewConsoleLogDispatcher(
	ps pubsub.PubSub,
	gateway ConsoleLogGateway,
	registry ConnectionChecker,
	instanceID string,
	logger *slog.Logger,
) ConsoleLogDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &consoleLogDispatcher{
		ps:          ps,
		gateway:     gateway,
		registry:    registry,
		instanceID:  instanceID,
		logger:      logger,
		pendingReqs: make(map[string]chan *messages.DaemonConsoleLogResponsePayload),
	}
}

func (d *consoleLogDispatcher) Start(ctx context.Context) error {
	if err := d.ps.Subscribe(ctx, channels.DaemonConsoleLogRequestAll, d.handleRequest); err != nil {
		return errors.Wrap(err, "subscribe to console log request dispatch")
	}

	responseChannel := channels.BuildDaemonConsoleLogResponseChannel(d.instanceID)
	if err := d.ps.Subscribe(ctx, responseChannel, d.handleResponse); err != nil {
		return errors.Wrap(err, "subscribe to console log response")
	}

	d.logger.Info("console log dispatcher started", "instance_id", d.instanceID)

	return nil
}

func (d *consoleLogDispatcher) DispatchConsoleLog(
	ctx context.Context, nodeID uint64, serverID uint64, maxBytes int64,
) (*proto.ConsoleLogResponse, error) {
	resp, err := d.dispatchAndWait(ctx, nodeID, messages.DaemonConsoleLogRequestPayload{
		NodeID:     nodeID,
		RequestID:  idgen.New(),
		InstanceID: d.instanceID,
		ServerID:   serverID,
		MaxBytes:   maxBytes,
	})
	if err != nil {
		return nil, err
	}

	var consoleResp proto.ConsoleLogResponse
	if err := consoleResp.UnmarshalVT(resp.Data); err != nil {
		return nil, errors.Wrap(err, "unmarshal console log response")
	}

	return &consoleResp, nil
}

func (d *consoleLogDispatcher) dispatchAndWait(
	ctx context.Context,
	nodeID uint64,
	payload messages.DaemonConsoleLogRequestPayload,
) (*messages.DaemonConsoleLogResponsePayload, error) {
	respCh := make(chan *messages.DaemonConsoleLogResponsePayload, 1)

	d.mu.Lock()
	d.pendingReqs[payload.RequestID] = respCh
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pendingReqs, payload.RequestID)
		d.mu.Unlock()
	}()

	channel := channels.BuildDaemonConsoleLogRequestChannel(nodeID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonConsoleLogRequest, payload)
	if err != nil {
		return nil, errors.WithMessage(err, "create console log request message")
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		return nil, errors.Wrap(err, "publish console log request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(consoleLogDispatchTimeout):
		return nil, errors.New("console log dispatch timed out")
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("console log dispatch request cancelled")
		}
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}

		return resp, nil
	}
}

func (d *consoleLogDispatcher) handleRequest(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonConsoleLogRequestPayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse console log request payload", "error", err)

		return nil
	}

	if !d.registry.IsConnected(payload.NodeID) {
		return nil
	}

	go d.executeAndRespond(payload) //nolint:gosec // intentionally outlives handler context

	return nil
}

func (d *consoleLogDispatcher) executeAndRespond(payload messages.DaemonConsoleLogRequestPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), consoleLogDispatchTimeout)
	defer cancel()

	resp := d.executeRequest(ctx, payload)
	resp.RequestID = payload.RequestID

	channel := channels.BuildDaemonConsoleLogResponseChannel(payload.InstanceID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonConsoleLogResponse, resp)
	if err != nil {
		d.logger.Error("failed to create console log response message", "error", err)

		return
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		d.logger.Error("failed to publish console log response",
			"request_id", payload.RequestID,
			"error", err,
		)
	}
}

func (d *consoleLogDispatcher) executeRequest(
	ctx context.Context,
	payload messages.DaemonConsoleLogRequestPayload,
) messages.DaemonConsoleLogResponsePayload {
	result, err := d.gateway.RequestConsoleLog(ctx, payload.NodeID, payload.ServerID, payload.MaxBytes)
	if err != nil {
		return messages.DaemonConsoleLogResponsePayload{Error: err.Error()}
	}

	data, err := result.MarshalVT()
	if err != nil {
		return messages.DaemonConsoleLogResponsePayload{Error: err.Error()}
	}

	return messages.DaemonConsoleLogResponsePayload{Data: data}
}

func (d *consoleLogDispatcher) handleResponse(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonConsoleLogResponsePayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse console log response payload", "error", err)

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
