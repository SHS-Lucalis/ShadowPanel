package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/pkg/idgen"
	"github.com/pkg/errors"
	goredis "github.com/redis/go-redis/v9"
)

type Redis struct {
	client     *goredis.Client
	pubsub     *goredis.PubSub
	handlers   map[string][]pubsub.Handler
	mu         sync.RWMutex
	logger     *slog.Logger
	instanceID string
	closed     bool
	closeOnce  sync.Once
	wg         sync.WaitGroup
	started    bool
}

type Config struct {
	Addr       string
	Password   string
	DB         int
	InstanceID string
}

func New(cfg Config) (*Redis, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Wrap(err, "failed to connect to Redis")
	}

	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID = idgen.New()
	}

	return &Redis{
		client:     client,
		handlers:   make(map[string][]pubsub.Handler),
		logger:     slog.Default(),
		instanceID: instanceID,
	}, nil
}

func NewFromClient(client *goredis.Client, instanceID string) *Redis {
	if instanceID == "" {
		instanceID = idgen.New()
	}

	return &Redis{
		client:     client,
		handlers:   make(map[string][]pubsub.Handler),
		logger:     slog.Default(),
		instanceID: instanceID,
	}
}

func (r *Redis) Publish(ctx context.Context, channel string, msg *pubsub.Message) error {
	r.mu.RLock()
	closed := r.closed
	r.mu.RUnlock()

	if closed {
		return pubsub.ErrClosed
	}

	msg.Source = r.instanceID

	data, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	if err := r.client.Publish(ctx, channel, data).Err(); err != nil {
		return errors.Wrap(err, "failed to publish message")
	}

	return nil
}

func (r *Redis) Subscribe(ctx context.Context, pattern string, handler pubsub.Handler) error {
	if pattern == "" {
		return pubsub.ErrEmptyPattern
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return pubsub.ErrClosed
	}

	r.handlers[pattern] = append(r.handlers[pattern], handler)

	if r.started && r.pubsub != nil {
		var err error
		if isPatternSubscription(pattern) {
			err = r.pubsub.PSubscribe(ctx, pattern)
		} else {
			err = r.pubsub.Subscribe(ctx, pattern)
		}

		if err != nil {
			return errors.Wrap(err, "failed to subscribe")
		}
	}

	return nil
}

func (r *Redis) Unsubscribe(ctx context.Context, pattern string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.handlers, pattern)

	if r.pubsub != nil {
		if isPatternSubscription(pattern) {
			return r.pubsub.PUnsubscribe(ctx, pattern)
		}

		return r.pubsub.Unsubscribe(ctx, pattern)
	}

	return nil
}

func (r *Redis) Start(ctx context.Context) error {
	r.mu.Lock()

	if r.closed {
		r.mu.Unlock()

		return pubsub.ErrClosed
	}

	r.pubsub = r.client.Subscribe(ctx)
	r.started = true

	for pattern := range r.handlers {
		var err error
		if isPatternSubscription(pattern) {
			err = r.pubsub.PSubscribe(ctx, pattern)
		} else {
			err = r.pubsub.Subscribe(ctx, pattern)
		}

		if err != nil {
			r.mu.Unlock()

			return errors.Wrap(err, "failed to subscribe")
		}
	}

	r.mu.Unlock()

	ch := r.pubsub.Channel()

	r.wg.Go(func() {
		r.processMessages(ctx, ch)
	})

	<-ctx.Done()

	return ctx.Err()
}

func (r *Redis) processMessages(ctx context.Context, ch <-chan *goredis.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}

			r.handleMessage(ctx, redisMsg)
		}
	}
}

func (r *Redis) handleMessage(ctx context.Context, redisMsg *goredis.Message) {
	var msg pubsub.Message
	if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		r.logger.Error("failed to unmarshal message",
			slog.String("channel", redisMsg.Channel),
			slog.String("error", err.Error()),
		)

		return
	}

	r.mu.RLock()
	handlers := r.getMatchingHandlers(redisMsg.Channel, redisMsg.Pattern)
	r.mu.RUnlock()

	for _, handler := range handlers {
		pubsub.SafeCall(ctx, handler, &msg, r.logger)
	}
}

func (r *Redis) getMatchingHandlers(channel, pattern string) []pubsub.Handler {
	var handlers []pubsub.Handler

	if h, ok := r.handlers[channel]; ok {
		handlers = append(handlers, h...)
	}

	if pattern != "" {
		if h, ok := r.handlers[pattern]; ok {
			handlers = append(handlers, h...)
		}
	}

	for p, h := range r.handlers {
		if p != channel && p != pattern && pubsub.MatchPattern(p, channel) {
			handlers = append(handlers, h...)
		}
	}

	return handlers
}

func (r *Redis) Close() error {
	var closeErr error

	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()

		if r.pubsub != nil {
			if err := r.pubsub.Close(); err != nil {
				closeErr = errors.Wrap(err, "failed to close pubsub")
			}
		}

		r.wg.Wait()
	})

	return closeErr
}

func isPatternSubscription(pattern string) bool {
	return strings.Contains(pattern, "*") ||
		strings.Contains(pattern, "?") ||
		strings.Contains(pattern, "[")
}
