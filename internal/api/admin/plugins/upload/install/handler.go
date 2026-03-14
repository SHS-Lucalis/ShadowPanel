package install

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/plugin"
	"github.com/gameap/gameap/internal/repositories"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
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
	manager     LoaderManager
	pluginRepo  repositories.PluginRepository
	fileManager files.FileManager
	loader      *plugin.Loader
	pluginsDir  string
	responder   base.Responder
}

func NewHandler(
	manager LoaderManager,
	pluginRepo repositories.PluginRepository,
	fileManager files.FileManager,
	loader *plugin.Loader,
	pluginsDir string,
	responder base.Responder,
) *Handler {
	return &Handler{
		manager:     manager,
		pluginRepo:  pluginRepo,
		fileManager: fileManager,
		loader:      loader,
		pluginsDir:  pluginsDir,
		responder:   responder,
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
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to load plugin for validation"))

		return
	}

	pluginID := pkgplugin.CompactPluginID(pkgplugin.ParsePluginID(loaded.Info.Id))
	dbID := pkgplugin.ParsePluginID(loaded.Info.Id)

	if err := h.manager.Unload(ctx, pluginID); err != nil {
		slog.WarnContext(ctx, "failed to unload temporary plugin",
			slog.String("plugin_id", pluginID),
			slog.String("error", err.Error()))
	}

	if err := h.checkNotInstalled(ctx, dbID); err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	filename := strconv.FormatUint(uint64(dbID), 10) + ".wasm"
	pluginPath := path.Join(h.pluginsDir, filename)

	if err := h.fileManager.Write(ctx, pluginPath, wasmBytes); err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to save plugin file"))

		return
	}

	pluginRecord := h.buildPluginRecord(dbID, loaded, filename)

	if err := h.pluginRepo.Save(ctx, pluginRecord); err != nil {
		_ = h.fileManager.Delete(ctx, pluginPath)
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to save plugin record"))

		return
	}

	h.tryLoadPlugin(ctx, pluginRecord, filename)

	h.responder.Write(ctx, rw, newInstallResponse(pluginRecord))
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

func (h *Handler) checkNotInstalled(ctx context.Context, dbID domain.Uint64ID) error {
	exists, err := h.pluginRepo.Exists(ctx, filters.FindPluginByIDs(dbID))
	if err != nil {
		return errors.WithMessage(err, "failed to check if plugin exists")
	}
	if exists {
		return errors.New("plugin already installed")
	}

	return nil
}

func (h *Handler) buildPluginRecord(
	dbID domain.Uint64ID,
	loaded *pkgplugin.LoadedPlugin,
	filename string,
) *domain.Plugin {
	return &domain.Plugin{
		ID:          dbID,
		Name:        loaded.Info.Name,
		Version:     loaded.Info.Version,
		Description: loaded.Info.Description,
		Author:      loaded.Info.Author,
		APIVersion:  loaded.Info.ApiVersion,
		Filename:    new(filename),
		Source:      new("file://" + filename),
		Status:      domain.PluginStatusActive,
		InstalledAt: new(time.Now()),
	}
}

func (h *Handler) tryLoadPlugin(ctx context.Context, pluginRecord *domain.Plugin, filename string) {
	if h.loader == nil {
		return
	}

	loaded, err := h.loader.LoadWithID(ctx, filename, uint64(pluginRecord.ID))
	if err != nil {
		slog.ErrorContext(ctx, "failed to load plugin",
			slog.String("filename", filename),
			slog.String("error", err.Error()))

		pluginRecord.Status = domain.PluginStatusError
		_ = h.pluginRepo.Save(ctx, pluginRecord)

		return
	}

	h.loader.RegisterPluginID(pluginRecord.ID, loaded.Info.Id)
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
