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

type AttachHandler struct {
	publisher pubsub.Publisher
	logger    *slog.Logger

	mu       sync.RWMutex
	sessions map[string]uint64 // session_id → server_id
}

func NewAttachHandler(publisher pubsub.Publisher, logger *slog.Logger) *AttachHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &AttachHandler{
		publisher: publisher,
		logger:    logger,
		sessions:  make(map[string]uint64),
	}
}

func (h *AttachHandler) HandleAttachStarted(ctx context.Context, nodeID uint64, started *proto.AttachStarted) error {
	h.logger.Debug("attach session started",
		"node_id", nodeID,
		"session_id", started.SessionId,
		"server_id", started.ServerId,
	)

	if h.publisher == nil {
		return nil
	}

	channel := channels.BuildRealtimeAttachStartedChannel(started.SessionId)

	msg, err := messages.NewMessage(channel, messages.TypeAttachStarted, messages.AttachStartedPayload{
		SessionID: started.SessionId,
		ServerID:  started.ServerId,
	})
	if err != nil {
		h.logger.Warn("failed to create attach started message", "error", err)

		return nil
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish attach started", "error", err)
	}

	return nil
}

func (h *AttachHandler) HandleAttachOutput(ctx context.Context, nodeID uint64, output *proto.AttachOutput) error {
	h.logger.Debug("attach output received",
		"node_id", nodeID,
		"session_id", output.SessionId,
		"bytes", len(output.Data),
	)

	if h.publisher == nil {
		return nil
	}

	channel := channels.BuildRealtimeAttachOutputChannel(output.SessionId)

	msg, err := messages.NewMessage(channel, messages.TypeAttachOutput, messages.AttachOutputPayload{
		SessionID: output.SessionId,
		Data:      output.Data,
	})
	if err != nil {
		h.logger.Warn("failed to create attach output message", "error", err)

		return nil
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish attach output", "error", err)
	}

	return nil
}

func (h *AttachHandler) HandleAttachClosed(ctx context.Context, nodeID uint64, closed *proto.AttachClosed) error {
	h.logger.Debug("attach session closed",
		"node_id", nodeID,
		"session_id", closed.SessionId,
		"reason", closed.Reason,
		"exit_code", closed.ExitCode,
	)

	h.mu.Lock()
	delete(h.sessions, closed.SessionId)
	h.mu.Unlock()

	if h.publisher == nil {
		return nil
	}

	channel := channels.BuildRealtimeAttachClosedChannel(closed.SessionId)

	msg, err := messages.NewMessage(channel, messages.TypeAttachClosed, messages.AttachClosedPayload{
		SessionID: closed.SessionId,
		Reason:    closed.Reason,
		ExitCode:  closed.ExitCode,
	})
	if err != nil {
		h.logger.Warn("failed to create attach closed message", "error", err)

		return nil
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish attach closed", "error", err)
	}

	return nil
}

func (h *AttachHandler) TrackAttachSession(sessionID string, serverID uint64) {
	h.mu.Lock()
	h.sessions[sessionID] = serverID
	h.mu.Unlock()
}

func (h *AttachHandler) UntrackAttachSession(sessionID string) {
	h.mu.Lock()
	delete(h.sessions, sessionID)
	h.mu.Unlock()
}

func (h *AttachHandler) SessionServerID(sessionID string) (uint64, bool) {
	h.mu.RLock()
	serverID, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	return serverID, ok
}
