package gameexporter

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/pkg/errors"
)

type Exporter struct {
	gameRepo    repositories.GameRepository
	gameModRepo repositories.GameModRepository
	version     string
}

func NewExporter(
	gameRepo repositories.GameRepository,
	gameModRepo repositories.GameModRepository,
	version string,
) *Exporter {
	return &Exporter{
		gameRepo:    gameRepo,
		gameModRepo: gameModRepo,
		version:     version,
	}
}

func (e *Exporter) Export(ctx context.Context, gameCode string) ([]byte, error) {
	if gameCode == "" {
		return nil, errors.New("game code is required")
	}

	games, err := e.gameRepo.Find(ctx, filters.FindGameByCodes(gameCode), nil, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find game")
	}

	if len(games) == 0 {
		return nil, errors.Errorf("game with code %q not found", gameCode)
	}

	game := &games[0]

	mods, err := e.gameModRepo.Find(
		ctx,
		filters.FindGameModByGameCodes(gameCode),
		nil,
		nil,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find game mods")
	}

	export := domain.NewGameExportFromDomain(game, mods, e.version)

	yamlData, err := export.ToYAML()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to marshal export to YAML")
	}

	return yamlData, nil
}

func (e *Exporter) ExportToStruct(ctx context.Context, gameCode string) (*domain.GameExport, error) {
	if gameCode == "" {
		return nil, errors.New("game code is required")
	}

	games, err := e.gameRepo.Find(ctx, filters.FindGameByCodes(gameCode), nil, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find game")
	}

	if len(games) == 0 {
		return nil, errors.Errorf("game with code %q not found", gameCode)
	}

	game := &games[0]

	mods, err := e.gameModRepo.Find(
		ctx,
		filters.FindGameModByGameCodes(gameCode),
		nil,
		nil,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find game mods")
	}

	return domain.NewGameExportFromDomain(game, mods, e.version), nil
}
