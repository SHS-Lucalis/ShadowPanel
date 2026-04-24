package handlers

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/proto"
)

type MetricsHandler struct {
	publisher pubsub.Publisher
	logger    *slog.Logger
}

func NewMetricsHandler(publisher pubsub.Publisher, logger *slog.Logger) *MetricsHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &MetricsHandler{
		publisher: publisher,
		logger:    logger,
	}
}

func (h *MetricsHandler) HandleMetricsBatch(
	ctx context.Context, nodeID uint64, batch *proto.MetricsBatch,
) error {
	if h.publisher == nil {
		return nil
	}

	data, err := batch.MarshalVT()
	if err != nil {
		h.logger.Warn("failed to marshal metrics batch",
			"node_id", nodeID,
			"error", err,
		)

		return nil
	}

	channel := channels.BuildRealtimeMetricsChannel(nodeID)

	msg, err := messages.NewMessage(channel, messages.TypeMetricsBatch, messages.MetricsBatchPayload{
		NodeID: nodeID,
		Data:   data,
	})
	if err != nil {
		h.logger.Warn("failed to create metrics batch message",
			"node_id", nodeID,
			"error", err,
		)

		return nil
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish metrics batch",
			"node_id", nodeID,
			"error", err,
		)
	}

	return nil
}
