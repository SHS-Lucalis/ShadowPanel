package plugin

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/services/servercontrol"
	"github.com/gameap/gameap/pkg/plugin/proto"
)

// ServerControlAdapter adapts the plugin.Dispatcher to the servercontrol.PluginDispatcher interface.
type ServerControlAdapter struct {
	dispatcher *Dispatcher
}

// NewServerControlAdapter creates a new ServerControlAdapter.
func NewServerControlAdapter(dispatcher *Dispatcher) *ServerControlAdapter {
	return &ServerControlAdapter{
		dispatcher: dispatcher,
	}
}

// DispatchServerEvent implements servercontrol.PluginDispatcher.
func (a *ServerControlAdapter) DispatchServerEvent(
	ctx context.Context,
	eventType servercontrol.PluginEventType,
	server *domain.Server,
	extraData map[string]string,
) *servercontrol.PluginDispatchResult {
	protoEventType := mapEventType(eventType)

	result := a.dispatcher.DispatchServerEvent(ctx, protoEventType, server, extraData)

	return &servercontrol.PluginDispatchResult{
		Cancelled:     result.Cancelled,
		CancelledBy:   result.CancelledBy,
		CancelMessage: result.CancelMessage,
	}
}

func mapEventType(eventType servercontrol.PluginEventType) proto.EventType {
	switch eventType {
	case servercontrol.PluginEventServerPreStart:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_START
	case servercontrol.PluginEventServerPostStart:
		return proto.EventType_EVENT_TYPE_SERVER_POST_START
	case servercontrol.PluginEventServerPreStop:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_STOP
	case servercontrol.PluginEventServerPostStop:
		return proto.EventType_EVENT_TYPE_SERVER_POST_STOP
	case servercontrol.PluginEventServerPreRestart:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_RESTART
	case servercontrol.PluginEventServerPostRestart:
		return proto.EventType_EVENT_TYPE_SERVER_POST_RESTART
	case servercontrol.PluginEventServerPreInstall:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_INSTALL
	case servercontrol.PluginEventServerPostInstall:
		return proto.EventType_EVENT_TYPE_SERVER_POST_INSTALL
	case servercontrol.PluginEventServerPreUpdate:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_UPDATE
	case servercontrol.PluginEventServerPostUpdate:
		return proto.EventType_EVENT_TYPE_SERVER_POST_UPDATE
	case servercontrol.PluginEventServerPreReinstall:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_REINSTALL
	case servercontrol.PluginEventServerPostReinstall:
		return proto.EventType_EVENT_TYPE_SERVER_POST_REINSTALL
	case servercontrol.PluginEventServerPreDelete:
		return proto.EventType_EVENT_TYPE_SERVER_PRE_DELETE
	case servercontrol.PluginEventServerPostDelete:
		return proto.EventType_EVENT_TYPE_SERVER_POST_DELETE
	default:
		return proto.EventType_EVENT_TYPE_UNSPECIFIED
	}
}
