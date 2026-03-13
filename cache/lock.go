package cache

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrLockNotAcquired is returned when a lock is already held elsewhere.
	ErrLockNotAcquired = errors.New("astra/lock: lock is already held")
	// ErrLockNotOwned is returned when a lock operation is attempted by a non-owner.
	ErrLockNotOwned = errors.New("astra/lock: lock is not owned by this instance")
)

// Locker acquires distributed locks.
type Locker interface {
	// Acquire attempts to acquire a lock with the provided TTL.
	Acquire(ctx context.Context, key string, ttl time.Duration, opts ...LockOption) (Lock, error)
}

// Lock represents an acquired distributed lock.
type Lock interface {
	// Release releases the lock if the caller still owns it.
	Release(ctx context.Context) error
	// Extend refreshes the lock TTL if the caller still owns it.
	Extend(ctx context.Context, ttl time.Duration) error
}

type lockOptions struct {
	retryAttempts int
	retryDelay    time.Duration
}

// LockOption configures lock acquisition behavior.
type LockOption func(*lockOptions)

// WithRetry retries lock acquisition a fixed number of times with a delay.
func WithRetry(attempts int, delay time.Duration) LockOption {
	return func(o *lockOptions) {
		if attempts < 0 {
			attempts = 0
		}
		if delay < 0 {
			delay = 0
		}
		o.retryAttempts = attempts
		o.retryDelay = delay
	}
}
