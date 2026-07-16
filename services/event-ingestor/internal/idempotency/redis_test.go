package idempotency

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestIdempotencyStore(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6380", // Docker-compose Redis mapping
	})
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		// Fallback check on standard port
		client = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		_, err = client.Ping(ctx).Result()
		if err != nil {
			t.Skip("Redis is not running on localhost:6380 or localhost:6379, skipping test")
		}
	}

	store := &Store{
		client: client,
		ttl:    5 * time.Second,
	}

	key := "test-idempotency-key"
	client.Del(ctx, key)
	defer client.Del(ctx, key)

	// 1. First acquire should succeed
	isNew, err := store.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("Expected no error on first Acquire: %v", err)
	}
	if !isNew {
		t.Error("Expected first Acquire to return true (isNew)")
	}

	// 2. Second acquire with same key should fail (duplicate)
	isNew2, err := store.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("Expected no error on second Acquire: %v", err)
	}
	if isNew2 {
		t.Error("Expected second Acquire to return false (duplicate)")
	}

	// 3. Release the key
	err = store.Release(ctx, key)
	if err != nil {
		t.Fatalf("Expected no error on Release: %v", err)
	}

	// 4. Third acquire should succeed again after release
	isNew3, err := store.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("Expected no error on third Acquire: %v", err)
	}
	if !isNew3 {
		t.Error("Expected Acquire to succeed after Release")
	}
}
