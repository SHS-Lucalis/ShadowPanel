package retry

import (
	"context"
	"log/slog"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/pkg/errors"
)

type DLQHandler interface {
	RecordFailure(ctx context.Context, msg *pubsub.Message, channel string, err error, attempts int) error
}

type Config struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
}

type Option func(*Publisher)

func WithDLQ(dlq DLQHandler) Option {
	return func(p *Publisher) {
		p.dlq = dlq
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(p *Publisher) {
		p.logger = logger
	}
}

type Publisher struct {
	publisher pubsub.Publisher
	config    Config
	dlq       DLQHandler
	logger    *slog.Logger
}

func NewPublisher(publisher pubsub.Publisher, cfg Config, opts ...Option) *Publisher {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 100 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 5 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	p := &Publisher{
		publisher: publisher,
		config:    cfg,
		logger:    slog.Default(),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Publisher) Publish(ctx context.Context, channel string, msg *pubsub.Message) error {
	var lastErr error
	delay := p.config.InitialDelay

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			delay = min(time.Duration(float64(delay)*p.config.Multiplier), p.config.MaxDelay)
		}

		if err := p.publisher.Publish(ctx, channel, msg); err != nil {
			if errors.Is(err, pubsub.ErrClosed) || errors.Is(err, pubsub.ErrPayloadTooLarge) {
				return err
			}
			lastErr = err
			p.logger.Warn("publish attempt failed, retrying",
				slog.String("channel", channel),
				slog.Int("attempt", attempt+1),
				slog.Int("max_retries", p.config.MaxRetries+1),
				slog.String("error", err.Error()),
			)

			continue
		}

		return nil
	}

	if p.dlq != nil {
		if dlqErr := p.dlq.RecordFailure(ctx, msg, channel, lastErr, p.config.MaxRetries+1); dlqErr != nil {
			p.logger.Error("failed to record message in DLQ",
				slog.String("channel", channel),
				slog.String("error", dlqErr.Error()),
			)
		}
	}

	return errors.Wrap(lastErr, "publish failed after retries")
}
