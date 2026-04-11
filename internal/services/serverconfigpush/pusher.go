package serverconfigpush

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/grpc/gateway"
	"github.com/gameap/gameap/internal/grpc/session"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/gameap/gameap/pkg/proto"
)

type Pusher struct {
	registry          *session.Registry
	serverRepo        repositories.ServerRepository
	serverSettingRepo repositories.ServerSettingRepository
	logger            *slog.Logger
}

func NewPusher(
	registry *session.Registry,
	serverRepo repositories.ServerRepository,
	serverSettingRepo repositories.ServerSettingRepository,
	logger *slog.Logger,
) *Pusher {
	if logger == nil {
		logger = slog.Default()
	}

	return &Pusher{
		registry:          registry,
		serverRepo:        serverRepo,
		serverSettingRepo: serverSettingRepo,
		logger:            logger,
	}
}

func (p *Pusher) PushServerConfig(ctx context.Context, serverID uint) {
	servers, err := p.serverRepo.Find(ctx, &filters.FindServer{
		IDs: []uint{serverID},
	}, nil, nil)
	if err != nil || len(servers) == 0 {
		p.logger.Warn("failed to load server for config push",
			"server_id", serverID,
			"error", err,
		)

		return
	}

	server := &servers[0]

	settings, err := p.serverSettingRepo.Find(ctx, &filters.FindServerSetting{
		ServerIDs: []uint{serverID},
	}, nil, nil)
	if err != nil {
		p.logger.Warn("failed to load server settings for config push",
			"server_id", serverID,
			"error", err,
		)
	}

	msg := &proto.GatewayMessage{
		RequestId: idgen.New(),
		Payload: &proto.GatewayMessage_ServerConfigUpdate{
			ServerConfigUpdate: &proto.ServerConfigUpdate{
				Server:   gateway.DomainServerToProto(server),
				Settings: gateway.DomainServerSettingsToProto(settings),
			},
		},
	}

	nodeID := uint64(server.DSID)

	if err := p.registry.SendTask(ctx, nodeID, msg); err != nil {
		p.logger.Debug("failed to push server config (node may not be connected)",
			"server_id", serverID,
			"node_id", nodeID,
			"error", err,
		)
	}
}
