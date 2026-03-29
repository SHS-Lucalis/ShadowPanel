package dlq

import (
	"context"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/pkg/errors"
)

type FailedMessage struct {
	ID           string          `json:"id"`
	OriginalMsg  *pubsub.Message `json:"original_msg"`
	Channel      string          `json:"channel"`
	Error        string          `json:"error"`
	AttemptCount int             `json:"attempt_count"`
	FailedAt     time.Time       `json:"failed_at"`
	Processed    bool            `json:"processed"`
	ProcessedAt  *time.Time      `json:"processed_at,omitempty"`
}

type Store interface {
	Push(ctx context.Context, msg *FailedMessage) error
	Pop(ctx context.Context) (*FailedMessage, error)
	List(ctx context.Context, limit, offset int) ([]FailedMessage, error)
	Count(ctx context.Context) (int, error)
	MarkProcessed(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	Purge(ctx context.Context) error
}

type Handler struct {
	store     Store
	publisher pubsub.Publisher
}

func NewHandler(store Store, publisher pubsub.Publisher) *Handler {
	return &Handler{
		store:     store,
		publisher: publisher,
	}
}

func (h *Handler) RecordFailure(
	ctx context.Context,
	msg *pubsub.Message,
	channel string,
	err error,
	attempts int,
) error {
	failedMsg := &FailedMessage{
		ID:           idgen.New(),
		OriginalMsg:  msg,
		Channel:      channel,
		Error:        err.Error(),
		AttemptCount: attempts,
		FailedAt:     time.Now(),
		Processed:    false,
	}

	return h.store.Push(ctx, failedMsg)
}

func (h *Handler) Reprocess(ctx context.Context, id string) error {
	msgs, err := h.store.List(ctx, 1000, 0)
	if err != nil {
		return errors.WithMessage(err, "failed to list messages")
	}

	var targetMsg *FailedMessage
	for i := range msgs {
		if msgs[i].ID == id {
			targetMsg = &msgs[i]

			break
		}
	}

	if targetMsg == nil {
		return errors.New("message not found")
	}

	if targetMsg.Processed {
		return errors.New("message already processed")
	}

	if err := h.publisher.Publish(ctx, targetMsg.Channel, targetMsg.OriginalMsg); err != nil {
		return errors.WithMessage(err, "failed to republish message")
	}

	return h.store.MarkProcessed(ctx, id)
}

func (h *Handler) ReprocessAll(ctx context.Context) (processed, failed int, err error) {
	attempted := make(map[string]struct{})

	for {
		msg, popErr := h.store.Pop(ctx)
		if popErr != nil {
			if errors.Is(popErr, ErrEmpty) {
				return processed, failed, nil
			}

			return processed, failed, errors.WithMessage(popErr, "failed to pop message")
		}

		if msg.Processed {
			continue
		}

		if _, alreadyAttempted := attempted[msg.ID]; alreadyAttempted {
			_ = h.store.Push(ctx, msg)

			return processed, failed, nil
		}
		attempted[msg.ID] = struct{}{}

		if pubErr := h.publisher.Publish(ctx, msg.Channel, msg.OriginalMsg); pubErr != nil {
			failed++
			_ = h.store.Push(ctx, msg)

			continue
		}

		processed++
	}
}

func (h *Handler) Store() Store {
	return h.store
}

var ErrEmpty = errors.New("dlq: queue is empty")
