package console

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/gameap/gameap/pkg/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

const defaultCommandTimeout = 300 * time.Second

func (h *Handler) newGRPCMessageHandler(
	ctx context.Context,
	client *ws.Client,
	server *domain.Server,
	node *domain.Node,
	user *domain.User,
	canSend bool,
) (ws.MessageHandler, func()) {
	var trackedCommands []string

	handler := func(_ context.Context, msg *ws.InboundMessage) {
		if msg.Type != typeConsoleCommand {
			return
		}

		if !canSend {
			client.SendMessage(ws.NewErrorMessage("permission denied: cannot send commands"))

			return
		}

		var payload consoleCommandPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}

		if payload.Command == "" {
			return
		}

		if err := h.abilityChecker.CheckOrError(
			ctx,
			user.ID,
			server.ID,
			[]domain.AbilityName{domain.AbilityNameGameServerConsoleSend},
		); err != nil {
			client.SendMessage(ws.NewErrorMessage("permission denied: cannot send commands"))

			return
		}

		commandID := idgen.New()
		h.commandHandler.TrackCommandServer(commandID, uint64(server.ID))
		trackedCommands = append(trackedCommands, commandID)

		cmd := &proto.CommandRequest{
			CommandId:    commandID,
			ServerId:     uint64(server.ID),
			Command:      payload.Command,
			Timeout:      durationpb.New(defaultCommandTimeout),
			StreamOutput: true,
		}

		if err := h.registry.SendCommand(ctx, uint64(node.ID), cmd); err != nil {
			h.logger.Warn("failed to send console command via gRPC",
				"server_id", server.ID,
				"error", err,
			)
			client.SendMessage(ws.NewErrorMessage("failed to send command"))
		}
	}

	cleanup := func() {
		for _, id := range trackedCommands {
			h.commandHandler.UntrackCommandServer(id)
		}
	}

	return handler, cleanup
}
