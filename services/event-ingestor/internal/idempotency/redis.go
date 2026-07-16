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

// Acquire atomically checks and sets the idempotency key using Redis SetNX.
// Returns true if this is a new event (key was set successfully).
// Returns false if the key already exists (duplicate event).
func (s *Store) Acquire(ctx context.Context, key string) (bool, error) {
	return s.client.SetNX(ctx, key, "1", s.ttl).Result()
}

// Release removes the idempotency key. Used to "undo" a reservation
// when a downstream operation (e.g., Kafka publish) fails, allowing
// the client to retry with the same idempotency key.
func (s *Store) Release(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}