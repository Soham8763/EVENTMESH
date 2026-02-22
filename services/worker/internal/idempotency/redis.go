package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
	ttl    time.Duration
}

func NewStore(addr string, ttl time.Duration) *Store {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &Store{
		client: rdb,
		ttl:    ttl,
	}
}

func (s *Store) Acquire(ctx context.Context, key string) (bool, error) {
	ok, err := s.client.SetNX(ctx, key, "1", s.ttl).Result()
	return ok, err
}

func (s *Store) AcquireLease(ctx context.Context, key string) (bool, error) {
	return s.client.SetNX(ctx, key, "leased", 30*time.Second).Result()
}

func (s *Store) ReleaseLease(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
