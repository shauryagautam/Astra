package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Lock represents a distributed lock.
type Lock struct {
	client redis.UniversalClient
	name   string
	token  string
}

// NewLock creates a new distributed lock.
func NewLock(client redis.UniversalClient, name string) *Lock {
	return &Lock{
		client: client,
		name:   "lock:" + name,
		token:  uuid.New().String(),
	}
}

// Acquire attempts to acquire the lock for the given duration.
func (l *Lock) Acquire(ctx context.Context, ttl time.Duration) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.name, l.token, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("cache: failed to acquire lock: %w", err)
	}
	return ok, nil
}

// Release releases the lock using a Lua script to ensure atomicity.
func (l *Lock) Release(ctx context.Context) (bool, error) {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	res, err := l.client.Eval(ctx, script, []string{l.name}, l.token).Result()
	if err != nil {
		return false, fmt.Errorf("cache: failed to release lock: %w", err)
	}
	return res == int64(1), nil
}

// WithLock attempts to acquire a lock, runs the provided callback if successful,
// and enforces cleanup of the lock afterwards. Retries up to the given timeout.
func (s *Store) WithLock(ctx context.Context, name string, lockTTL time.Duration, timeout time.Duration, fn func() error) error {
	lock := NewLock(s.client, name)

	deadline := time.Now().Add(timeout)
	for {
		acquired, err := lock.Acquire(ctx, lockTTL)
		if err != nil {
			return err
		}
		if acquired {
			defer lock.Release(ctx)
			return fn()
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("cache: timeout acquiring lock %s", name)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
