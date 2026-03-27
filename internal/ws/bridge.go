package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
)

type Bridge struct {
	hub    *Hub
	ps     pubsub.Subscriber
	logger *slog.Logger
}

func NewBridge(hub *Hub, ps pubsub.Subscriber, logger *slog.Logger) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}

	return &Bridge{
		hub:    hub,
		ps:     ps,
		logger: logger,
	}
}

func (b *Bridge) Start(ctx context.Context) error {
	patterns := []string{
		channels.RealtimeTaskAll,
		channels.RealtimeConsoleAll,
		channels.RealtimeAttachAll,
	}

	for _, pattern := range patterns {
		if err := b.ps.Subscribe(ctx, pattern, b.handleMessage); err != nil {
			return err
		}
	}

	b.logger.Info("websocket bridge started", "patterns", patterns)

	return nil
}

func (b *Bridge) handleMessage(_ context.Context, msg *pubsub.Message) error {
	b.logger.Info("bridge received pubsub message",
		"channel", msg.Channel,
		"type", msg.Type,
	)

	topic := ChannelToTopic(msg.Channel)
	if topic == "" {
		return nil
	}

	envelope, err := wrapPayload(msg.Type, msg.Payload)
	if err != nil {
		b.logger.Warn("failed to wrap message payload", "error", err)

		return nil
	}

	b.hub.Broadcast(topic, envelope)

	return nil
}

func wrapPayload(msgType string, payload []byte) ([]byte, error) {
	msg := NewOutboundMessage(msgType, json.RawMessage(payload))

	return json.Marshal(msg)
}

func ChannelToTopic(channel string) string {
	return strings.TrimPrefix(channel, channels.Prefix)
}
