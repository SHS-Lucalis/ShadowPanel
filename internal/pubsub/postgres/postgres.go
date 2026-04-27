package postgres

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

const (
	maxPayloadSize = 7900

	// transportChannel is the single PostgreSQL LISTEN/NOTIFY channel used to
	// route every pub-sub message. Pattern matching happens in Go via
	// pubsub.MatchPattern because PG LISTEN/NOTIFY only supports exact
	// channel-name matching.
	transportChannel = "gameap_pubsub"
)

type Postgres struct {
	pool              *pgxpool.Pool
	connStr           string
	handlers          map[string][]pubsub.Handler
	mu                sync.RWMutex
	logger            *slog.Logger
	instanceID        string
	closed            bool
	closeOnce         sync.Once
	wg                sync.WaitGroup
	reconnectInterval time.Duration
	maxReconnectDelay time.Duration
}

type Config struct {
	ConnStr           string
	InstanceID        string
	ReconnectInterval time.Duration
	MaxReconnectDelay time.Duration
}

func New(cfg Config) (*Postgres, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.ConnStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse connection string")
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create connection pool")
	}

	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID = idgen.New()
	}

	reconnectInterval := cfg.ReconnectInterval
	if reconnectInterval == 0 {
		reconnectInterval = 5 * time.Second
	}

	maxReconnectDelay := cfg.MaxReconnectDelay
	if maxReconnectDelay == 0 {
		maxReconnectDelay = 2 * time.Minute
	}

	return &Postgres{
		pool:              pool,
		connStr:           cfg.ConnStr,
		handlers:          make(map[string][]pubsub.Handler),
		logger:            slog.Default(),
		instanceID:        instanceID,
		reconnectInterval: reconnectInterval,
		maxReconnectDelay: maxReconnectDelay,
	}, nil
}

func (p *Postgres) Publish(ctx context.Context, _ string, msg *pubsub.Message) error {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		return pubsub.ErrClosed
	}

	msg.Source = p.instanceID

	data, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	if len(data) > maxPayloadSize {
		return pubsub.ErrPayloadTooLarge
	}

	_, err = p.pool.Exec(ctx, "SELECT pg_notify($1, $2)", transportChannel, string(data))
	if err != nil {
		return errors.Wrap(err, "failed to send notification")
	}

	return nil
}

func (p *Postgres) Subscribe(_ context.Context, pattern string, handler pubsub.Handler) error {
	if pattern == "" {
		return pubsub.ErrEmptyPattern
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return pubsub.ErrClosed
	}

	p.handlers[pattern] = append(p.handlers[pattern], handler)

	return nil
}

func (p *Postgres) Unsubscribe(_ context.Context, pattern string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.handlers, pattern)

	return nil
}

func (p *Postgres) Start(ctx context.Context) error {
	p.wg.Go(func() {
		p.listenLoop(ctx)
	})

	<-ctx.Done()

	return ctx.Err()
}

func (p *Postgres) listenLoop(ctx context.Context) {
	delay := p.reconnectInterval

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := p.listen(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}

			p.logger.Error("listener error, reconnecting",
				slog.String("error", err.Error()),
				slog.Duration("delay", delay),
			)

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
				delay = min(delay*2, p.maxReconnectDelay)
			}
		} else {
			delay = p.reconnectInterval
		}
	}
}

func (p *Postgres) listen(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, p.connStr)
	if err != nil {
		return errors.Wrap(err, "failed to connect")
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+transportChannel); err != nil {
		return errors.Wrapf(err, "failed to listen on channel %s", transportChannel)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		notification, err := conn.WaitForNotification(waitCtx)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				continue
			}

			return errors.Wrap(err, "notification error")
		}

		p.handleNotification(ctx, notification)
	}
}

func (p *Postgres) handleNotification(ctx context.Context, notification *pgconn.Notification) {
	var msg pubsub.Message
	if err := json.Unmarshal([]byte(notification.Payload), &msg); err != nil {
		p.logger.Error("failed to unmarshal notification",
			slog.String("channel", notification.Channel),
			slog.String("error", err.Error()),
		)

		return
	}

	p.mu.RLock()
	handlers := p.getMatchingHandlers(msg.Channel)
	p.mu.RUnlock()

	for _, handler := range handlers {
		pubsub.SafeCall(ctx, handler, &msg, p.logger)
	}
}

func (p *Postgres) getMatchingHandlers(channel string) []pubsub.Handler {
	var handlers []pubsub.Handler

	for pattern, h := range p.handlers {
		if pubsub.MatchPattern(pattern, channel) {
			handlers = append(handlers, h...)
		}
	}

	return handlers
}

func (p *Postgres) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()

		p.pool.Close()
		p.wg.Wait()
	})

	return nil
}
