package dryrun

import (
	"context"
	"io"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
	"github.com/gameap/gameap/pkg/plugin/proto"
	"github.com/pkg/errors"
)

const (
	maxMemory     = 32 << 20  // 32 MB
	maxUploadSize = 100 << 20 // 100 MB
)

var (
	errNoFileUploaded   = errors.New("no file uploaded")
	errFileTooSmall     = errors.New("file too small to be valid WASM")
	errInvalidWASMMagic = errors.New("invalid WASM magic number")
)

type LoaderManager interface {
	Load(ctx context.Context, wasmBytes []byte, config map[string]string, pluginID uint64) (*pkgplugin.LoadedPlugin, error)
	Unload(ctx context.Context, pluginID string) error
}

type Handler struct {
	manager   LoaderManager
	responder base.Responder
}

func NewHandler(
	manager LoaderManager,
	responder base.Responder,
) *Handler {
	return &Handler{
		manager:   manager,
		responder: responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	wasmBytes, err := h.readWASMFile(rw, r)
	if err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	if err := validateWASM(wasmBytes); err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	loaded, err := h.manager.Load(ctx, wasmBytes, nil, 0)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to load plugin"))

		return
	}

	pluginID := pkgplugin.CompactPluginID(pkgplugin.ParsePluginID(loaded.Info.Id))

	subscribedEvents := h.getSubscribedEvents(ctx, loaded)

	if err := h.manager.Unload(ctx, pluginID); err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to unload plugin"))

		return
	}

	h.responder.Write(ctx, rw, newDryRunResponse(loaded, subscribedEvents))
}

func (h *Handler) readWASMFile(rw http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(rw, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, errors.WithMessage(err, "failed to parse multipart form")
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, errNoFileUploaded
	}
	defer func() { _ = file.Close() }()

	wasmBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read uploaded file")
	}

	return wasmBytes, nil
}

func (h *Handler) getSubscribedEvents(ctx context.Context, loaded *pkgplugin.LoadedPlugin) []proto.EventType {
	resp, err := loaded.Instance.GetSubscribedEvents(ctx, &proto.GetSubscribedEventsRequest{})
	if err != nil {
		return nil
	}

	return resp.Events
}

func validateWASM(data []byte) error {
	if len(data) < 4 {
		return errFileTooSmall
	}
	// WASM magic number: \x00asm
	if data[0] != 0x00 || data[1] != 0x61 || data[2] != 0x73 || data[3] != 0x6d {
		return errInvalidWASMMagic
	}

	return nil
}
