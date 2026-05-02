package uploadsession

import (
	"context"
	"io"
	"net/http"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/upload"
	"github.com/gameap/gameap/pkg/api"
)

type Service interface {
	Create(ctx context.Context, p upload.CreateParams) (*upload.Session, error)
	WriteChunk(ctx context.Context, uploadID string, userID uint, index uint, body io.Reader) error
	Status(ctx context.Context, uploadID string, userID uint) (*upload.SessionStatus, error)
	Complete(ctx context.Context, uploadID string, userID uint, node *domain.Node) error
	Abort(ctx context.Context, uploadID string, userID uint) error
}

var ErrUploadIDRequired = api.NewError(http.StatusBadRequest, "uploadID is required")
