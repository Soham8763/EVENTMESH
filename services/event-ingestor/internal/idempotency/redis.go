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

func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	res, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

func (s *Store) Set(ctx context.Context, key string) error {
	return s.client.Set(ctx, key, "1", s.ttl).Err()
}