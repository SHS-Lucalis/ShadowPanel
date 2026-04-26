package handlers

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/proto"
)

type MetricsHandler struct {
	publisher pubsub.Publisher
	logger    *slog.Logger

	waitersMu sync.Mutex
	waiters   map[string]metricsWaiter
}

type metricsWaiter struct {
	nodeID           uint64
	remoteInstanceID string
}

func NewMetricsHandler(publisher pubsub.Publisher, logger *slog.Logger) *MetricsHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &MetricsHandler{
		publisher: publisher,
		logger:    logger,
		waiters:   make(map[string]metricsWaiter),
	}
}

func (h *MetricsHandler) RegisterPollWaiter(requestID string, nodeID uint64) {
	h.waitersMu.Lock()
	defer h.waitersMu.Unlock()

	h.waiters[requestID] = metricsWaiter{nodeID: nodeID}
}

func (h *MetricsHandler) RegisterRemoteWaiter(requestID string, nodeID uint64, requesterInstanceID string) {
	h.waitersMu.Lock()
	defer h.waitersMu.Unlock()

	h.waiters[requestID] = metricsWaiter{
		nodeID:           nodeID,
		remoteInstanceID: requesterInstanceID,
	}
}

func (h *MetricsHandler) CancelWaiter(requestID string) {
	h.waitersMu.Lock()
	defer h.waitersMu.Unlock()

	delete(h.waiters, requestID)
}

func (h *MetricsHandler) HandleMetricsResponse(
	ctx context.Context, nodeID uint64, requestID string, resp *proto.MetricsResponse,
) error {
	h.waitersMu.Lock()
	waiter, ok := h.waiters[requestID]
	if ok {
		delete(h.waiters, requestID)
	}
	h.waitersMu.Unlock()

	if !ok {
		h.logger.Warn("metrics response with unknown request_id",
			"node_id", nodeID,
			"request_id", requestID,
		)

		return nil
	}

	if h.publisher == nil {
		return nil
	}

	data, err := resp.MarshalVT()
	if err != nil {
		h.logger.Warn("failed to marshal metrics response",
			"node_id", nodeID,
			"error", err,
		)

		return nil
	}

	if waiter.remoteInstanceID == "" {
		return h.publishLiveFanout(ctx, nodeID, data)
	}

	return h.publishRemoteReply(ctx, waiter.remoteInstanceID, requestID, nodeID, data)
}

func (h *MetricsHandler) publishLiveFanout(ctx context.Context, nodeID uint64, data []byte) error {
	channel := channels.BuildRealtimeMetricsChannel(nodeID)

	msg, err := messages.NewMessage(channel, messages.TypeMetricsLive, messages.MetricsLivePayload{
		NodeID: nodeID,
		Data:   data,
	})
	if err != nil {
		h.logger.Warn("failed to create metrics live message",
			"node_id", nodeID,
			"error", err,
		)

		return nil
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish metrics live",
			"node_id", nodeID,
			"error", err,
		)
	}

	return nil
}

func (h *MetricsHandler) publishRemoteReply(
	ctx context.Context, requesterInstanceID, requestID string, nodeID uint64, data []byte,
) error {
	channel := channels.BuildDaemonMetricsResponseChannel(requesterInstanceID)

	msg, err := messages.NewMessage(channel, messages.TypeDaemonMetricsResponse, messages.DaemonMetricsResponsePayload{
		RequestID: requestID,
		NodeID:    nodeID,
		Data:      data,
	})
	if err != nil {
		h.logger.Warn("failed to create metrics reply message",
			"node_id", nodeID,
			"error", err,
		)

		return nil
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish metrics reply",
			"node_id", nodeID,
			"request_id", requestID,
			"error", err,
		)
	}

	return nil
}
