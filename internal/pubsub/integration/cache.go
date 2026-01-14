package integration

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
)

type CacheInvalidator struct {
	pubsub pubsub.PubSub
	cache  cache.Cache
	logger *slog.Logger
}

func NewCacheInvalidator(ps pubsub.PubSub, c cache.Cache) *CacheInvalidator {
	return &CacheInvalidator{
		pubsub: ps,
		cache:  c,
		logger: slog.Default(),
	}
}

func (ci *CacheInvalidator) Start(ctx context.Context) error {
	return ci.pubsub.Subscribe(ctx, channels.CacheInvalidateAll, ci.handleInvalidation)
}

func (ci *CacheInvalidator) PublishInvalidation(ctx context.Context, entityType string, entityIDs ...string) error {
	payload := messages.CacheInvalidatePayload{
		EntityType: entityType,
		EntityIDs:  entityIDs,
	}

	msg, err := messages.NewMessage(
		channels.BuildCacheInvalidateChannel(entityType, ""),
		messages.TypeCacheInvalidate,
		payload,
	)
	if err != nil {
		return err
	}

	return ci.pubsub.Publish(ctx, channels.CacheInvalidate, msg)
}

func (ci *CacheInvalidator) handleInvalidation(ctx context.Context, msg *pubsub.Message) error {
	payload, err := messages.ParsePayload[messages.CacheInvalidatePayload](msg)
	if err != nil {
		ci.logger.Error("failed to parse cache invalidation payload",
			slog.String("error", err.Error()),
		)

		return nil
	}

	ci.logger.Debug("received cache invalidation",
		slog.String("entity_type", payload.EntityType),
		slog.Any("entity_ids", payload.EntityIDs),
	)

	if redisCache, ok := ci.cache.(*cache.Redis); ok {
		pattern := payload.EntityType + ":*"

		return redisCache.DeletePattern(ctx, pattern)
	}

	return ci.cache.Clear(ctx)
}
