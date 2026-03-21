package importpelicanegg

import (
	"context"
	"io"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/domain/gamesimport"
	"github.com/gameap/gameap/internal/services/pelicaneggimporter"
	"github.com/gameap/gameap/pkg/api"
	"github.com/pkg/errors"
)

type EggImporter interface {
	Import(
		ctx context.Context,
		egg *gamesimport.PelicanEgg,
		opts *gamesimport.Options,
	) (*pelicaneggimporter.ImportResult, error)
}

type Handler struct {
	importer  EggImporter
	responder base.Responder
}

func NewHandler(
	importer EggImporter,
	responder base.Responder,
) *Handler {
	return &Handler{
		importer:  importer,
		responder: responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	inp, err := readInput(r)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.Wrap(err, "failed to read input"))

		return
	}

	opts := inp.toImportOptions()
	if err := opts.Validate(); err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(err, http.StatusBadRequest))

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.Wrap(err, "failed to read request body"))

		return
	}

	egg, err := gamesimport.ParsePelicanEgg(body)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to parse pelican egg"))

		return
	}

	result, err := h.importer.Import(ctx, egg, opts)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to import pelican egg"))

		return
	}

	h.responder.Write(ctx, rw, Response{
		GameCode:  result.Game.Code,
		GameName:  result.Game.Name,
		GameModID: result.GameMod.ID,
	})
}

type Response struct {
	GameCode  string `json:"game_code"`
	GameName  string `json:"game_name"`
	GameModID uint   `json:"game_mod_id"`
}
