package integration

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/plugin/proto"
	"github.com/samber/lo"
)

type PluginEventPublisher struct {
	pubsub pubsub.Publisher
	logger *slog.Logger
}

func NewPluginEventPublisher(ps pubsub.Publisher) *PluginEventPublisher {
	return &PluginEventPublisher{
		pubsub: ps,
		logger: slog.Default(),
	}
}

func (p *PluginEventPublisher) PublishEvent(ctx context.Context, event *proto.Event) error {
	payload := messages.PluginEventPayload{
		EventType: int32(event.Type),
	}

	if serverEvent := event.GetServerEvent(); serverEvent != nil && serverEvent.Server != nil {
		payload.ServerID = lo.ToPtr(uint(serverEvent.Server.Id))
		payload.ExtraData = serverEvent.ExtraData
	}

	if taskEvent := event.GetTaskEvent(); taskEvent != nil {
		payload.TaskID = lo.ToPtr(uint(taskEvent.TaskId))
		payload.NodeID = lo.ToPtr(uint(taskEvent.NodeId))

		if taskEvent.ServerId != nil {
			payload.ServerID = lo.ToPtr(uint(*taskEvent.ServerId))
		}

		payload.ExtraData = taskEvent.ExtraData
	}

	channel := channels.PluginEvents
	if payload.ServerID != nil {
		channel = channels.PluginServerEvents
	} else if payload.TaskID != nil {
		channel = channels.PluginTaskEvents
	}

	msg, err := messages.NewMessage(channel, messages.TypePluginEvent, payload)
	if err != nil {
		return err
	}

	return p.pubsub.Publish(ctx, channel, msg)
}

func (p *PluginEventPublisher) PublishServerEvent(
	ctx context.Context,
	eventType proto.EventType,
	serverID uint,
	extraData map[string]string,
) error {
	payload := messages.PluginEventPayload{
		EventType: int32(eventType),
		ServerID:  lo.ToPtr(serverID),
		ExtraData: extraData,
	}

	msg, err := messages.NewMessage(channels.PluginServerEvents, messages.TypePluginEvent, payload)
	if err != nil {
		return err
	}

	return p.pubsub.Publish(ctx, channels.PluginServerEvents, msg)
}

func (p *PluginEventPublisher) PublishTaskEvent(
	ctx context.Context,
	eventType proto.EventType,
	taskID, nodeID uint,
	serverID *uint,
	extraData map[string]string,
) error {
	payload := messages.PluginEventPayload{
		EventType: int32(eventType),
		TaskID:    lo.ToPtr(taskID),
		NodeID:    lo.ToPtr(nodeID),
		ServerID:  serverID,
		ExtraData: extraData,
	}

	msg, err := messages.NewMessage(channels.PluginTaskEvents, messages.TypePluginEvent, payload)
	if err != nil {
		return err
	}

	return p.pubsub.Publish(ctx, channels.PluginTaskEvents, msg)
}
