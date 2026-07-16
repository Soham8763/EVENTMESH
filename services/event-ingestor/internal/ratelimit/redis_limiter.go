package ratelimit

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	client       *redis.Client
	defaultLimit int64
	window       time.Duration
}

func NewLimiter(client *redis.Client) *Limiter {
	limitStr := os.Getenv("DEFAULT_TENANT_RATE_LIMIT") // e.g. 1000 requests per minute
	limit := int64(1000)
	if limitStr != "" {
		if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
			limit = parsed
		}
	}

	return &Limiter{
		client:       client,
		defaultLimit: limit,
		window:       time.Minute,
	}
}

func (l *Limiter) Allow(ctx context.Context, tenantID string) (bool, error) {
	now := time.Now()
	nowMs := now.UnixNano() / int64(time.Millisecond)
	clearBefore := now.Add(-l.window).UnixNano() / int64(time.Millisecond)

	key := "ratelimit:tenant:" + tenantID

	// Multi/Exec transaction to make it atomic
	pipe := l.client.TxPipeline()

	// 1. Remove timestamps older than the sliding window
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(clearBefore, 10))

	// 2. Count requests in the current window
	cardCmd := pipe.ZCard(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := cardCmd.Val()

	if count >= l.defaultLimit {
		return false, nil
	}

	// 3. Add the current timestamp and set expiration on the key
	pipe2 := l.client.TxPipeline()
	nowStr := strconv.FormatInt(nowMs, 10)
	pipe2.ZAdd(ctx, key, redis.Z{Score: float64(nowMs), Member: nowStr})
	pipe2.Expire(ctx, key, l.window)

	_, err = pipe2.Exec(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
