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

const commandDispatchTimeout = 30 * time.Second

type commandDispatcher struct {
	ps         pubsub.PubSub
	gateway    CommandGateway
	registry   ConnectionChecker
	instanceID string
	logger     *slog.Logger

	mu          sync.RWMutex
	pendingReqs map[string]chan *messages.DaemonCommandResponsePayload
}

func NewCommandDispatcher(
	ps pubsub.PubSub,
	gateway CommandGateway,
	registry ConnectionChecker,
	instanceID string,
	logger *slog.Logger,
) CommandDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &commandDispatcher{
		ps:          ps,
		gateway:     gateway,
		registry:    registry,
		instanceID:  instanceID,
		logger:      logger,
		pendingReqs: make(map[string]chan *messages.DaemonCommandResponsePayload),
	}
}

func (d *commandDispatcher) Start(ctx context.Context) error {
	if err := d.ps.Subscribe(ctx, channels.DaemonCommandRequestAll, d.handleCommandRequest); err != nil {
		return errors.Wrap(err, "subscribe to command request dispatch")
	}

	responseChannel := channels.BuildDaemonCommandResponseChannel(d.instanceID)
	if err := d.ps.Subscribe(ctx, responseChannel, d.handleCommandResponse); err != nil {
		return errors.Wrap(err, "subscribe to command response")
	}

	d.logger.Info("command dispatcher started", "instance_id", d.instanceID)

	return nil
}

func (d *commandDispatcher) DispatchCommand(
	ctx context.Context, nodeID uint64, req *proto.CommandRequest,
) (*proto.CommandResult, error) {
	reqData, err := req.MarshalVT()
	if err != nil {
		return nil, errors.Wrap(err, "marshal command request")
	}

	resp, err := d.dispatchAndWait(ctx, nodeID, messages.DaemonCommandRequestPayload{
		NodeID:     nodeID,
		RequestID:  idgen.New(),
		InstanceID: d.instanceID,
		Data:       reqData,
	})
	if err != nil {
		return nil, err
	}

	var cmdResult proto.CommandResult
	if err := cmdResult.UnmarshalVT(resp.Data); err != nil {
		return nil, errors.Wrap(err, "unmarshal command result")
	}

	return &cmdResult, nil
}

func (d *commandDispatcher) dispatchAndWait(
	ctx context.Context,
	nodeID uint64,
	payload messages.DaemonCommandRequestPayload,
) (*messages.DaemonCommandResponsePayload, error) {
	respCh := make(chan *messages.DaemonCommandResponsePayload, 1)

	d.mu.Lock()
	d.pendingReqs[payload.RequestID] = respCh
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pendingReqs, payload.RequestID)
		d.mu.Unlock()
	}()

	channel := channels.BuildDaemonCommandRequestChannel(nodeID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonCommandRequest, payload)
	if err != nil {
		return nil, errors.WithMessage(err, "create command request message")
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		return nil, errors.Wrap(err, "publish command request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(commandDispatchTimeout):
		return nil, errors.New("command dispatch timed out")
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("command dispatch request cancelled")
		}
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}

		return resp, nil
	}
}

func (d *commandDispatcher) handleCommandRequest(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonCommandRequestPayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse command request payload", "error", err)

		return nil
	}

	if !d.registry.IsConnected(payload.NodeID) {
		return nil
	}

	go d.executeAndRespond(payload) //nolint:gosec // intentionally outlives handler context

	return nil
}

func (d *commandDispatcher) executeAndRespond(payload messages.DaemonCommandRequestPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), commandDispatchTimeout)
	defer cancel()

	resp := d.executeCommandRequest(ctx, payload)
	resp.RequestID = payload.RequestID

	channel := channels.BuildDaemonCommandResponseChannel(payload.InstanceID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonCommandResponse, resp)
	if err != nil {
		d.logger.Error("failed to create command response message", "error", err)

		return
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		d.logger.Error("failed to publish command response",
			"request_id", payload.RequestID,
			"error", err,
		)
	}
}

func (d *commandDispatcher) executeCommandRequest(
	ctx context.Context,
	payload messages.DaemonCommandRequestPayload,
) messages.DaemonCommandResponsePayload {
	var req proto.CommandRequest
	if err := req.UnmarshalVT(payload.Data); err != nil {
		return messages.DaemonCommandResponsePayload{Error: err.Error()}
	}

	result, err := d.gateway.RequestCommand(ctx, payload.NodeID, &req)
	if err != nil {
		return messages.DaemonCommandResponsePayload{Error: err.Error()}
	}

	data, err := result.MarshalVT()
	if err != nil {
		return messages.DaemonCommandResponsePayload{Error: err.Error()}
	}

	return messages.DaemonCommandResponsePayload{Data: data}
}

func (d *commandDispatcher) handleCommandResponse(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonCommandResponsePayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse command response payload", "error", err)

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
