package putgamemod

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	gmBase "github.com/gameap/gameap/internal/api/gamemods/base"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/services/serverconfigpush"
	"github.com/gameap/gameap/pkg/api"
	"github.com/pkg/errors"
)

var ErrGameModNotFound = api.NewNotFoundError("game mod not found")

type Handler struct {
	repo         repositories.GameModRepository
	serverRepo   repositories.ServerRepository
	configPusher *serverconfigpush.Pusher
	responder    base.Responder
}

func NewHandler(
	repo repositories.GameModRepository,
	serverRepo repositories.ServerRepository,
	configPusher *serverconfigpush.Pusher,
	responder base.Responder,
) *Handler {
	return &Handler{
		repo:         repo,
		serverRepo:   serverRepo,
		configPusher: configPusher,
		responder:    responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := api.NewInputReader(r).ReadUint("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid game mod id"),
			http.StatusBadRequest,
		))

		return
	}

	input := &updateGameModInput{}

	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "invalid request"))

		return
	}

	err = input.Validate()
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "invalid input"))

		return
	}

	gameMods, err := h.repo.Find(ctx, &filters.FindGameMod{
		IDs: []uint{id},
	}, nil, &filters.Pagination{
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find game mod"))

		return
	}

	if len(gameMods) == 0 {
		h.responder.WriteError(ctx, rw, ErrGameModNotFound)

		return
	}

	gameMod := &gameMods[0]

	input.Apply(gameMod)

	err = h.repo.Save(ctx, gameMod)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to update game mod"))

		return
	}

	h.pushConfigForGameModServers(ctx, gameMod.ID)

	h.responder.Write(ctx, rw, gmBase.NewGameModResponseFromGameMod(gameMod))
}

func (h *Handler) pushConfigForGameModServers(ctx context.Context, gameModID uint) {
	if h.configPusher == nil || h.serverRepo == nil {
		return
	}

	servers, err := h.serverRepo.Find(ctx, &filters.FindServer{
		GameModIDs: []uint{gameModID},
	}, nil, nil)
	if err != nil {
		slog.Default().Warn("failed to load servers for game mod config push",
			"game_mod_id", gameModID,
			"error", err,
		)

		return
	}

	for i := range servers {
		h.configPusher.PushServerConfig(ctx, servers[i].ID)
	}
}
