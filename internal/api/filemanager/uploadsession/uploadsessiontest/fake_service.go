package uploadsessiontest

import (
	"context"
	"io"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/pkg/errors"

	"github.com/gameap/gameap/internal/upload"
)

// errFuncNotSet is returned by every FakeService method whose hook the test
// did not configure. Returning a loud error rather than a silent zero value
// means a handler that unexpectedly reaches the service layer fails the test
// instead of producing a misleading "200 OK".
var errFuncNotSet = errors.New("fake upload service func not set")

type FakeService struct {
	CreateFunc   func(context.Context, upload.CreateParams) (*upload.Session, error)
	WriteFunc    func(context.Context, string, uint, uint, io.Reader) error
	StatusFunc   func(context.Context, string, uint) (*upload.SessionStatus, error)
	CompleteFunc func(context.Context, string, uint, *domain.Node) error
	AbortFunc    func(context.Context, string, uint) error
}

func (f *FakeService) Create(ctx context.Context, p upload.CreateParams) (*upload.Session, error) {
	if f.CreateFunc != nil {
		return f.CreateFunc(ctx, p)
	}

	return nil, errFuncNotSet
}

func (f *FakeService) WriteChunk(ctx context.Context, id string, uid, idx uint, b io.Reader) error {
	if f.WriteFunc != nil {
		return f.WriteFunc(ctx, id, uid, idx, b)
	}

	return errFuncNotSet
}

func (f *FakeService) Status(ctx context.Context, id string, uid uint) (*upload.SessionStatus, error) {
	if f.StatusFunc != nil {
		return f.StatusFunc(ctx, id, uid)
	}

	return nil, errFuncNotSet
}

func (f *FakeService) Complete(ctx context.Context, id string, uid uint, node *domain.Node) error {
	if f.CompleteFunc != nil {
		return f.CompleteFunc(ctx, id, uid, node)
	}

	return errFuncNotSet
}

func (f *FakeService) Abort(ctx context.Context, id string, uid uint) error {
	if f.AbortFunc != nil {
		return f.AbortFunc(ctx, id, uid)
	}

	return errFuncNotSet
}

// EmptyService returns a fake service whose every method returns errFuncNotSet
// when called. Use it for table cases that should never reach the service
// layer (e.g. unauthorized, forbidden, malformed input). If a refactor lets a
// "should not call service" path slip through, the request will fail loudly.
func EmptyService(*testing.T) *FakeService {
	return &FakeService{}
}
