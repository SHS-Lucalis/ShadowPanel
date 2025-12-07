package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/gamemods"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type GameModsServiceImpl struct {
	gameModRepo repositories.GameModRepository
}

func NewGameModsService(gameModRepo repositories.GameModRepository) *GameModsServiceImpl {
	return &GameModsServiceImpl{
		gameModRepo: gameModRepo,
	}
}

func (s *GameModsServiceImpl) FindGameMods(
	ctx context.Context,
	req *gamemods.FindGameModsRequest,
) (*gamemods.FindGameModsResponse, error) {
	var filter *filters.FindGameMod
	if req.Filter != nil {
		filter = &filters.FindGameMod{
			IDs: uintsFromUint64s(req.Filter.Ids),
		}
		if req.Filter.GameCode != nil {
			filter.GameCodes = []string{*req.Filter.GameCode}
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

	result, err := s.gameModRepo.Find(ctx, filter, sorting, pagination)
	if err != nil {
		return nil, err
	}

	return &gamemods.FindGameModsResponse{
		GameMods: convertGameModsToProto(result),
		Total:    int32(len(result)), //nolint:gosec
	}, nil
}

func (s *GameModsServiceImpl) GetGameMod(
	ctx context.Context,
	req *gamemods.GetGameModRequest,
) (*gamemods.GetGameModResponse, error) {
	result, err := s.gameModRepo.Find(ctx, &filters.FindGameMod{IDs: []uint{uint(req.Id)}}, nil, nil)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return &gamemods.GetGameModResponse{Found: false}, nil
	}

	return &gamemods.GetGameModResponse{
		GameMod: convertGameModToProto(&result[0]),
		Found:   true,
	}, nil
}

func convertGameModsToProto(gms []domain.GameMod) []*proto.GameMod {
	return lo.Map(gms, func(gm domain.GameMod, _ int) *proto.GameMod {
		return convertGameModToProto(&gm)
	})
}

func convertGameModToProto(gm *domain.GameMod) *proto.GameMod {
	return &proto.GameMod{
		Id:              uint64(gm.ID),
		GameCode:        gm.GameCode,
		Name:            gm.Name,
		StartCmdLinux:   gm.StartCmdLinux,
		StartCmdWindows: gm.StartCmdWindows,
		KickCmd:         gm.KickCmd,
		BanCmd:          gm.BanCmd,
	}
}

type GameModsHostLibrary struct {
	impl *GameModsServiceImpl
}

func NewGameModsHostLibrary(gameModRepo repositories.GameModRepository) *GameModsHostLibrary {
	return &GameModsHostLibrary{
		impl: NewGameModsService(gameModRepo),
	}
}

func (l *GameModsHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return gamemods.Instantiate(ctx, r, l.impl)
}
