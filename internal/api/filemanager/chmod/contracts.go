package chmod

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
)

type fileService interface {
	Chmod(ctx context.Context, node *domain.Node, path string, perm uint32) error
}
