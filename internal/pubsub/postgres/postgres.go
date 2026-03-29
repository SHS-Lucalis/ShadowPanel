package postgres

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

const maxPayloadSize = 7900

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

	started       bool
	subRequests   chan subRequest
	unsubRequests chan string
}

type subRequest struct {
	pattern string
	done    chan error
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
		subRequests:       make(chan subRequest),
		unsubRequests:     make(chan string),
	}, nil
}

func (p *Postgres) Publish(ctx context.Context, channel string, msg *pubsub.Message) error {
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

	pgChannel := sanitizeChannelName(channel)

	_, err = p.pool.Exec(ctx, "SELECT pg_notify($1, $2)", pgChannel, string(data))
	if err != nil {
		return errors.Wrap(err, "failed to send notification")
	}

	return nil
}

func (p *Postgres) Subscribe(ctx context.Context, pattern string, handler pubsub.Handler) error {
	if pattern == "" {
		return pubsub.ErrEmptyPattern
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()

		return pubsub.ErrClosed
	}
	p.handlers[pattern] = append(p.handlers[pattern], handler)
	started := p.started
	p.mu.Unlock()

	if started {
		req := subRequest{pattern: pattern, done: make(chan error, 1)}
		select {
		case p.subRequests <- req:
			return <-req.done
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (p *Postgres) Unsubscribe(ctx context.Context, pattern string) error {
	p.mu.Lock()
	delete(p.handlers, pattern)
	started := p.started
	p.mu.Unlock()

	if started {
		select {
		case p.unsubRequests <- pattern:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (p *Postgres) Start(ctx context.Context) error {
	p.mu.Lock()
	p.started = true
	p.mu.Unlock()

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

	listeningChannels := make(map[string]struct{})
	channels := p.getListenChannels()
	for _, ch := range channels {
		pgChannel := sanitizeChannelName(ch)

		_, err := conn.Exec(ctx, "LISTEN "+pgChannel)
		if err != nil {
			return errors.Wrapf(err, "failed to listen on channel %s", pgChannel)
		}
		listeningChannels[pgChannel] = struct{}{}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case req := <-p.subRequests:
			pgChannel := sanitizeChannelName(getBaseChannel(req.pattern))
			if _, exists := listeningChannels[pgChannel]; !exists {
				_, err := conn.Exec(ctx, "LISTEN "+pgChannel)
				if err != nil {
					req.done <- errors.Wrapf(err, "failed to listen on channel %s", pgChannel)
				} else {
					listeningChannels[pgChannel] = struct{}{}
					req.done <- nil
				}
			} else {
				req.done <- nil
			}
		case pattern := <-p.unsubRequests:
			pgChannel := sanitizeChannelName(getBaseChannel(pattern))
			if _, exists := listeningChannels[pgChannel]; exists {
				_, _ = conn.Exec(ctx, "UNLISTEN "+pgChannel)
				delete(listeningChannels, pgChannel)
			}
		default:
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

func (p *Postgres) getListenChannels() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	channelSet := make(map[string]struct{})
	for pattern := range p.handlers {
		baseChannel := getBaseChannel(pattern)
		channelSet[baseChannel] = struct{}{}
	}

	channels := make([]string, 0, len(channelSet))
	for ch := range channelSet {
		channels = append(channels, ch)
	}

	return channels
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

func sanitizeChannelName(channel string) string {
	r := strings.ReplaceAll(channel, ":", "__")
	r = strings.ReplaceAll(r, ".", "_")
	r = strings.ReplaceAll(r, "-", "_")
	r = strings.TrimSuffix(r, "*")

	return r
}

func getBaseChannel(pattern string) string {
	if before, _, found := strings.Cut(pattern, "*"); found {
		return before
	}

	return pattern
}
