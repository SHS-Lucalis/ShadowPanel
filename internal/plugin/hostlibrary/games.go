package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/games"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type GamesServiceImpl struct {
	gameRepo repositories.GameRepository
}

func NewGamesService(gameRepo repositories.GameRepository) *GamesServiceImpl {
	return &GamesServiceImpl{
		gameRepo: gameRepo,
	}
}

func (s *GamesServiceImpl) FindGames(
	ctx context.Context,
	req *games.FindGamesRequest,
) (*games.FindGamesResponse, error) {
	var filter *filters.FindGame
	if req.Filter != nil {
		filter = &filters.FindGame{
			Codes: req.Filter.Codes,
		}
	}

	var pagination *filters.Pagination
	if req.Pagination != nil {
		pagination = &filters.Pagination{
			Limit:  int(req.Pagination.Limit),
			Offset: int(req.Pagination.Offset),
		}
	}

	sorting := convertSorting(req.Sorting)

	result, err := s.gameRepo.Find(ctx, filter, sorting, pagination)
	if err != nil {
		return nil, err
	}

	return &games.FindGamesResponse{
		Games: convertGamesToProto(result),
		Total: int32(len(result)), //nolint:gosec
	}, nil
}

func (s *GamesServiceImpl) GetGame(
	ctx context.Context,
	req *games.GetGameRequest,
) (*games.GetGameResponse, error) {
	result, err := s.gameRepo.Find(ctx, filters.FindGameByCodes(req.Code), nil, nil)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return &games.GetGameResponse{Found: false}, nil
	}

	return &games.GetGameResponse{
		Game:  convertGameToProto(&result[0]),
		Found: true,
	}, nil
}

func convertGamesToProto(gms []domain.Game) []*proto.Game {
	return lo.Map(gms, func(g domain.Game, _ int) *proto.Game {
		return convertGameToProto(&g)
	})
}

func convertGameToProto(g *domain.Game) *proto.Game {
	var steamAppIDLinux, steamAppIDWindows *uint32
	if g.SteamAppIDLinux != nil {
		steamAppIDLinux = lo.ToPtr(uint32(*g.SteamAppIDLinux)) //nolint:gosec
	}
	if g.SteamAppIDWindows != nil {
		steamAppIDWindows = lo.ToPtr(uint32(*g.SteamAppIDWindows)) //nolint:gosec
	}

	return &proto.Game{
		Code:                    g.Code,
		Name:                    g.Name,
		Engine:                  g.Engine,
		EngineVersion:           g.EngineVersion,
		SteamAppIdLinux:         steamAppIDLinux,
		SteamAppIdWindows:       steamAppIDWindows,
		SteamAppSetConfig:       g.SteamAppSetConfig,
		RemoteRepositoryLinux:   g.RemoteRepositoryLinux,
		RemoteRepositoryWindows: g.RemoteRepositoryWindows,
		Enabled:                 g.Enabled != 0,
	}
}

type GamesHostLibrary struct {
	impl *GamesServiceImpl
}

func NewGamesHostLibrary(gameRepo repositories.GameRepository) *GamesHostLibrary {
	return &GamesHostLibrary{
		impl: NewGamesService(gameRepo),
	}
}

func (l *GamesHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return games.Instantiate(ctx, r, l.impl)
}
