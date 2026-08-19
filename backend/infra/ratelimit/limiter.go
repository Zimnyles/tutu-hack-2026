package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "ratelimit:"

type Limiter struct {
	client *redis.Client
}

func New(client *redis.Client) *Limiter {
	return &Limiter{client: client}
}

func (l *Limiter) Allow(
	ctx context.Context,
	scope string,
	identity string,
	limit int,
	window time.Duration,
) (bool, error) {
	if limit <= 0 {
		return false, nil
	}

	key := fmt.Sprintf("%s%s:%s:%d", keyPrefix, scope, identity, time.Now().UnixNano()/int64(window))

	pipeline := l.client.TxPipeline()
	counter := pipeline.Incr(ctx, key)
	pipeline.Expire(ctx, key, window)

	if _, err := pipeline.Exec(ctx); err != nil {
		return true, fmt.Errorf("increment rate limit counter: %w", err)
	}

	return counter.Val() <= int64(limit), nil
}

func (l *Limiter) Reset(ctx context.Context, scope string, identity string, window time.Duration) error {
	key := fmt.Sprintf("%s%s:%s:%d", keyPrefix, scope, identity, time.Now().UnixNano()/int64(window))
	if err := l.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("reset rate limit counter: %w", err)
	}

	return nil
}
