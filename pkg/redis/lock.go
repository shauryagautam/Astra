package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrLockAcquisitionTimeout = errors.New("redis: lock acquisition timeout")
	ErrLockReleaseFailed      = errors.New("redis: lock release failed")
)

// Lock represents an advanced distributed lock.
type Lock struct {
	client redis.UniversalClient
	name   string
	token  string
	ttl    time.Duration
}

// NewLock creates a new Lock instance.
func (c *Client) NewLock(name string, ttl time.Duration) *Lock {
	return &Lock{
		client: c.UniversalClient,
		name:   "lock:" + name,
		token:  uuid.New().String(),
		ttl:    ttl,
	}
}

// Acquire attempts to acquire the lock.
func (l *Lock) Acquire(ctx context.Context) (bool, error) {
	return l.client.SetNX(ctx, l.name, l.token, l.ttl).Result()
}

// Release releases the lock if it's held by this instance.
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
		return false, fmt.Errorf("%w: %v", ErrLockReleaseFailed, err)
	}
	return res == int64(1), nil
}

// Extend extends the lock's TTL if it's still held by this instance.
func (l *Lock) Extend(ctx context.Context, additionalTTL time.Duration) (bool, error) {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	res, err := l.client.Eval(ctx, script, []string{l.name}, l.token, additionalTTL.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	return res == int64(1), nil
}

// WithLock executes the given function while holding the lock.
// It automatically retries for the given timeout and handles release.
func (c *Client) WithLock(ctx context.Context, name string, ttl time.Duration, timeout time.Duration, fn func(ctx context.Context) error) error {
	lock := c.NewLock(name, ttl)

	deadline := time.Now().Add(timeout)
	for {
		acquired, err := lock.Acquire(ctx)
		if err != nil {
			return err
		}
		if acquired {
			defer lock.Release(ctx)

			// Setup auto-renewal if TTL is long enough
			if ttl > 5*time.Second {
				renewCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				go lock.autoRenew(renewCtx, ttl)
			}

			return fn(ctx)
		}

		if time.Now().After(deadline) {
			return ErrLockAcquisitionTimeout
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// autoRenew periodically extends the lock TTL until context is cancelled.
func (l *Lock) autoRenew(ctx context.Context, ttl time.Duration) {
	ticker := time.NewTicker(ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			extended, _ := l.Extend(ctx, ttl)
			if !extended {
				return
			}
		}
	}
}
