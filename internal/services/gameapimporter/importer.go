package gameapimporter

import (
	"context"
	"maps"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/domain/gamesimport"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/pkg/errors"
)

type transactionManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) (err error)
}

type Importer struct {
	gameRepo    repositories.GameRepository
	gameModRepo repositories.GameModRepository
	tm          transactionManager
}

func NewImporter(
	gameRepo repositories.GameRepository,
	gameModRepo repositories.GameModRepository,
	tm transactionManager,
) *Importer {
	return &Importer{
		gameRepo:    gameRepo,
		gameModRepo: gameModRepo,
		tm:          tm,
	}
}

type ImportResult struct {
	Game        *domain.Game
	ModsCreated []string
	ModsUpdated []string
}

func (i *Importer) Import(ctx context.Context, export *gamesimport.GameExport) (*ImportResult, error) {
	if export == nil {
		return nil, errors.New("export cannot be nil")
	}

	if err := export.Validate(); err != nil {
		return nil, errors.WithMessage(err, "validation failed")
	}

	game := export.Game.ToDomainGame()

	mods := make([]*domain.GameMod, 0, len(export.Mods))
	for _, modExport := range export.Mods {
		mod := modExport.ToDomainGameMod(game.Code)
		mods = append(mods, mod)
	}

	var result ImportResult

	err := i.tm.Do(ctx, func(ctx context.Context) error {
		existingGames, err := i.gameRepo.Find(
			ctx,
			filters.FindGameByCodes(game.Code),
			nil,
			nil,
		)
		if err != nil {
			return errors.WithMessage(err, "failed to find existing game")
		}

		if len(existingGames) > 0 {
			game.Metadata = mergeMetadata(existingGames[0].Metadata, game.Metadata)
		}

		if err := i.gameRepo.Save(ctx, game); err != nil {
			return errors.WithMessage(err, "failed to save game")
		}

		result.Game = game

		for _, mod := range mods {
			existingMods, err := i.gameModRepo.Find(
				ctx,
				&filters.FindGameMod{
					Names:     []string{mod.Name},
					GameCodes: []string{game.Code},
				},
				nil,
				nil,
			)
			if err != nil {
				return errors.WithMessage(err, "failed to find existing game mod")
			}

			if len(existingMods) > 0 {
				mod.ID = existingMods[0].ID
				mod.Metadata = mergeMetadata(existingMods[0].Metadata, mod.Metadata)
				result.ModsUpdated = append(result.ModsUpdated, mod.Name)
			} else {
				result.ModsCreated = append(result.ModsCreated, mod.Name)
			}

			if err := i.gameModRepo.Save(ctx, mod); err != nil {
				return errors.WithMessage(err, "failed to save game mod")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func mergeMetadata(existing, updated domain.Metadata) domain.Metadata {
	if existing == nil {
		return updated
	}

	if updated == nil {
		return existing
	}

	result := make(domain.Metadata)
	maps.Copy(result, existing)
	maps.Copy(result, updated)

	return result
}
