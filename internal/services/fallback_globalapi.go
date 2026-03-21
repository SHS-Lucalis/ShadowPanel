package services

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/gameap/gameap/internal/domain"
	"github.com/pkg/errors"
)

//go:embed data/games.json
var fallbackGamesJSON []byte

type FallbackGlobalAPIService struct{}

func NewFallbackGlobalAPIService() *FallbackGlobalAPIService {
	return &FallbackGlobalAPIService{}
}

func (s *FallbackGlobalAPIService) Games(_ context.Context) ([]domain.GlobalAPIGame, error) {
	var apiResp domain.GlobalAPIResponse[[]domain.GlobalAPIGame]
	if err := json.Unmarshal(fallbackGamesJSON, &apiResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode fallback games data")
	}

	return apiResp.Data, nil
}
