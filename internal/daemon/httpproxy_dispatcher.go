package daemon

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const (
	httpProxyDispatchTimeout = 60 * time.Second
	httpProxyStoragePrefix   = "httpproxy/"
	// storageThreshold is the max size of serialized response data
	// that can be sent inline via PubSub. Larger responses are stored
	// in shared storage and referenced by path. This is necessary
	// because Postgres NOTIFY has a ~7.9KB payload limit.
	storageThreshold = 4 * 1024
)

type httpProxyDispatcher struct {
	ps         pubsub.PubSub
	gateway    HTTPProxyGateway
	registry   ConnectionChecker
	storage    files.StreamFileManager
	instanceID string
	logger     *slog.Logger

	mu          sync.RWMutex
	pendingReqs map[string]chan *messages.DaemonHTTPProxyResponsePayload
}

func NewHTTPProxyDispatcher(
	ps pubsub.PubSub,
	gateway HTTPProxyGateway,
	registry ConnectionChecker,
	storage files.StreamFileManager,
	instanceID string,
	logger *slog.Logger,
) HTTPProxyDispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &httpProxyDispatcher{
		ps:          ps,
		gateway:     gateway,
		registry:    registry,
		storage:     storage,
		instanceID:  instanceID,
		logger:      logger,
		pendingReqs: make(map[string]chan *messages.DaemonHTTPProxyResponsePayload),
	}
}

func (d *httpProxyDispatcher) Start(ctx context.Context) error {
	if err := d.ps.Subscribe(ctx, channels.DaemonHTTPProxyRequestAll, d.handleRequest); err != nil {
		return errors.Wrap(err, "subscribe to http proxy request dispatch")
	}

	responseChannel := channels.BuildDaemonHTTPProxyResponseChannel(d.instanceID)
	if err := d.ps.Subscribe(ctx, responseChannel, d.handleResponse); err != nil {
		return errors.Wrap(err, "subscribe to http proxy response")
	}

	d.logger.Info("http proxy dispatcher started", "instance_id", d.instanceID)

	return nil
}

func (d *httpProxyDispatcher) DispatchHTTPProxy(
	ctx context.Context, nodeID uint64, req *proto.HTTPProxyRequest,
) (*proto.HTTPProxyResponse, error) {
	reqData, err := req.MarshalVT()
	if err != nil {
		return nil, errors.Wrap(err, "marshal http proxy request")
	}

	resp, err := d.dispatchAndWait(ctx, nodeID, messages.DaemonHTTPProxyRequestPayload{
		NodeID:     nodeID,
		RequestID:  idgen.New(),
		InstanceID: d.instanceID,
		Data:       reqData,
	})
	if err != nil {
		return nil, err
	}

	data, err := d.resolveResponseData(ctx, resp)
	if err != nil {
		return nil, errors.WithMessage(err, "resolve response data")
	}

	var proxyResp proto.HTTPProxyResponse
	if err := proxyResp.UnmarshalVT(data); err != nil {
		return nil, errors.Wrap(err, "unmarshal http proxy response")
	}

	return &proxyResp, nil
}

func (d *httpProxyDispatcher) resolveResponseData(
	ctx context.Context,
	resp *messages.DaemonHTTPProxyResponsePayload,
) ([]byte, error) {
	if resp.StoragePath == "" {
		return resp.Data, nil
	}

	reader, err := d.storage.ReadStream(ctx, resp.StoragePath)
	if err != nil {
		return nil, errors.Wrap(err, "read from storage")
	}
	defer func() {
		_ = reader.Close()
	}()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "read storage data")
	}

	_ = d.storage.Delete(context.Background(), resp.StoragePath)

	return data, nil
}

func (d *httpProxyDispatcher) dispatchAndWait(
	ctx context.Context,
	nodeID uint64,
	payload messages.DaemonHTTPProxyRequestPayload,
) (*messages.DaemonHTTPProxyResponsePayload, error) {
	respCh := make(chan *messages.DaemonHTTPProxyResponsePayload, 1)

	d.mu.Lock()
	d.pendingReqs[payload.RequestID] = respCh
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pendingReqs, payload.RequestID)
		d.mu.Unlock()
	}()

	channel := channels.BuildDaemonHTTPProxyRequestChannel(nodeID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonHTTPProxyRequest, payload)
	if err != nil {
		return nil, errors.WithMessage(err, "create http proxy request message")
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		return nil, errors.Wrap(err, "publish http proxy request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(httpProxyDispatchTimeout):
		return nil, errors.New("http proxy dispatch timed out")
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("http proxy dispatch request cancelled")
		}
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}

		return resp, nil
	}
}

func (d *httpProxyDispatcher) handleRequest(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonHTTPProxyRequestPayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse http proxy request payload", "error", err)

		return nil
	}

	if !d.registry.IsConnected(payload.NodeID) {
		return nil
	}

	go d.executeAndRespond(payload) //nolint:gosec // intentionally outlives handler context

	return nil
}

func (d *httpProxyDispatcher) executeAndRespond(payload messages.DaemonHTTPProxyRequestPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), httpProxyDispatchTimeout)
	defer cancel()

	resp := d.executeRequest(ctx, payload)
	resp.RequestID = payload.RequestID

	if len(resp.Data) > storageThreshold {
		storagePath := httpProxyStoragePrefix + payload.RequestID
		if err := d.storage.WriteStream(ctx, storagePath, bytes.NewReader(resp.Data)); err != nil {
			d.logger.Error("failed to write proxy response to storage",
				"request_id", payload.RequestID,
				"error", err,
			)
		} else {
			resp.Data = nil
			resp.StoragePath = storagePath
		}
	}

	channel := channels.BuildDaemonHTTPProxyResponseChannel(payload.InstanceID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonHTTPProxyResponse, resp)
	if err != nil {
		d.logger.Error("failed to create http proxy response message", "error", err)

		return
	}

	if err := d.ps.Publish(ctx, channel, msg); err != nil {
		d.logger.Error("failed to publish http proxy response",
			"request_id", payload.RequestID,
			"error", err,
		)
	}
}

func (d *httpProxyDispatcher) executeRequest(
	ctx context.Context,
	payload messages.DaemonHTTPProxyRequestPayload,
) messages.DaemonHTTPProxyResponsePayload {
	var req proto.HTTPProxyRequest
	if err := req.UnmarshalVT(payload.Data); err != nil {
		return messages.DaemonHTTPProxyResponsePayload{Error: err.Error()}
	}

	result, err := d.gateway.RequestHTTPProxy(ctx, payload.NodeID, &req)
	if err != nil {
		return messages.DaemonHTTPProxyResponsePayload{Error: err.Error()}
	}

	data, err := result.MarshalVT()
	if err != nil {
		return messages.DaemonHTTPProxyResponsePayload{Error: err.Error()}
	}

	return messages.DaemonHTTPProxyResponsePayload{Data: data}
}

func (d *httpProxyDispatcher) handleResponse(_ context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.DaemonHTTPProxyResponsePayload](msg)
	if err != nil {
		d.logger.Warn("failed to parse http proxy response payload", "error", err)

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
