package inmemory

import (
	"context"

	"github.com/gameap/gameap/internal/pubsub/dlq"
)

type DLQRepository struct {
	store *dlq.MemoryStore
}

func NewDLQRepository(maxSize int) *DLQRepository {
	return &DLQRepository{
		store: dlq.NewMemoryStore(maxSize),
	}
}

func (r *DLQRepository) Push(ctx context.Context, msg *dlq.FailedMessage) error {
	return r.store.Push(ctx, msg)
}

func (r *DLQRepository) Pop(ctx context.Context) (*dlq.FailedMessage, error) {
	return r.store.Pop(ctx)
}

func (r *DLQRepository) List(ctx context.Context, limit, offset int) ([]dlq.FailedMessage, error) {
	return r.store.List(ctx, limit, offset)
}

func (r *DLQRepository) Count(ctx context.Context) (int, error) {
	return r.store.Count(ctx)
}

func (r *DLQRepository) MarkProcessed(ctx context.Context, id string) error {
	return r.store.MarkProcessed(ctx, id)
}

func (r *DLQRepository) Delete(ctx context.Context, id string) error {
	return r.store.Delete(ctx, id)
}

func (r *DLQRepository) Purge(ctx context.Context) error {
	return r.store.Purge(ctx)
}
