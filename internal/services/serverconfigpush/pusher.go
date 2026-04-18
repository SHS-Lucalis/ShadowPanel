package serverconfigpush

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/internal/domain"
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
	gameRepo          repositories.GameRepository
	gameModRepo       repositories.GameModRepository
	nodeRepo          repositories.NodeRepository
	logger            *slog.Logger
}

func NewPusher(
	registry *session.Registry,
	serverRepo repositories.ServerRepository,
	serverSettingRepo repositories.ServerSettingRepository,
	gameRepo repositories.GameRepository,
	gameModRepo repositories.GameModRepository,
	nodeRepo repositories.NodeRepository,
	logger *slog.Logger,
) *Pusher {
	if logger == nil {
		logger = slog.Default()
	}

	return &Pusher{
		registry:          registry,
		serverRepo:        serverRepo,
		serverSettingRepo: serverSettingRepo,
		gameRepo:          gameRepo,
		gameModRepo:       gameModRepo,
		nodeRepo:          nodeRepo,
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

	var gameMod *domain.GameMod
	gameMods, gmErr := p.gameModRepo.Find(ctx, &filters.FindGameMod{
		IDs: []uint{server.GameModID},
	}, nil, nil)
	if gmErr != nil {
		p.logger.Warn("failed to load game mod for config push",
			"game_mod_id", server.GameModID,
			"error", gmErr,
		)
	} else if len(gameMods) > 0 {
		gameMod = &gameMods[0]
	}

	var game *domain.Game
	games, gErr := p.gameRepo.Find(ctx, filters.FindGameByCodes(server.GameID), nil, nil)
	if gErr != nil {
		p.logger.Warn("failed to load game for config push",
			"game_id", server.GameID,
			"error", gErr,
		)
	} else if len(games) > 0 {
		game = &games[0]
	}

	var nodeOS domain.NodeOS
	nodes, nodeErr := p.nodeRepo.Find(ctx, &filters.FindNode{
		IDs: []uint{server.DSID},
	}, nil, nil)
	if nodeErr != nil {
		p.logger.Warn("failed to load node for config push",
			"node_id", server.DSID,
			"error", nodeErr,
		)
	} else if len(nodes) > 0 {
		nodeOS = nodes[0].OS
	}

	settings, err := p.serverSettingRepo.Find(ctx, &filters.FindServerSetting{
		ServerIDs: []uint{serverID},
	}, nil, nil)
	if err != nil {
		p.logger.Warn("failed to load server settings for config push",
			"server_id", serverID,
			"error", err,
		)
	}

	update := &proto.ServerConfigUpdate{
		Server:   gateway.DomainServerToProtoWithGameMod(server, gameMod, nodeOS),
		Settings: gateway.DomainServerSettingsToProto(settings),
	}

	if game != nil {
		update.Game = gateway.DomainGameToProto(game)
	}
	if gameMod != nil {
		update.GameMod = gateway.DomainGameModToProto(gameMod)
	}

	msg := &proto.GatewayMessage{
		RequestId: idgen.New(),
		Payload: &proto.GatewayMessage_ServerConfigUpdate{
			ServerConfigUpdate: update,
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
