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

// AcquireLease attempts to acquire a short-lived lease for a task.
// The lease auto-expires after 30 seconds, allowing another worker
// to pick up the task if this worker crashes mid-execution.
func (s *Store) AcquireLease(ctx context.Context, key string) (bool, error) {
	return s.client.SetNX(ctx, key, "leased", 30*time.Second).Result()
}

// ReleaseLease explicitly releases the lease after task completion.
func (s *Store) ReleaseLease(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// IsDone checks whether a task has been permanently marked as completed.
// This is the idempotency guard — if a task was already completed
// successfully, it should never be executed again.
func (s *Store) IsDone(ctx context.Context, key string) (bool, error) {
	res, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

// MarkDone permanently marks a task as completed. This key has a long TTL
// (24 hours) to prevent re-execution of completed tasks during that window.
func (s *Store) MarkDone(ctx context.Context, key string) error {
	return s.client.Set(ctx, key, "done", 24*time.Hour).Err()
}
