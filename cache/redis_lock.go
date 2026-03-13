package cache

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const defaultRedisLockPrefix = "astra:lock:"

var redisLockReleaseScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
end
return 0
`)

var redisLockExtendScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

// RedisLocker acquires distributed locks backed by Redis.
type RedisLocker struct {
	client    goredis.UniversalClient
	keyPrefix string
}

// RedisLock is a Redis-backed distributed lock.
type RedisLock struct {
	client goredis.UniversalClient
	key    string
	token  string
}

// NewRedisLocker creates a new Redis-backed locker.
func NewRedisLocker(client goredis.UniversalClient, keyPrefix string) *RedisLocker {
	return &RedisLocker{
		client:    client,
		keyPrefix: normalizeLockPrefix(keyPrefix),
	}
}

// Acquire attempts to acquire a distributed lock.
func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration, opts ...LockOption) (Lock, error) {
	if l.client == nil {
		return nil, fmt.Errorf("astra/lock: redis client is nil")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("astra/lock: ttl must be greater than zero")
	}

	options := lockOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	token, err := generateLockToken()
	if err != nil {
		return nil, fmt.Errorf("astra/lock: %w", err)
	}

	lockKey := l.keyPrefix + key
	for attempt := 0; attempt <= options.retryAttempts; attempt++ {
		acquired, acquireErr := l.client.SetNX(ctx, lockKey, token, ttl).Result()
		if acquireErr != nil {
			return nil, fmt.Errorf("astra/lock: %w", acquireErr)
		}
		if acquired {
			return &RedisLock{
				client: l.client,
				key:    lockKey,
				token:  token,
			}, nil
		}
		if attempt == options.retryAttempts {
			break
		}

		timer := time.NewTimer(options.retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, ErrLockNotAcquired
}

// Release releases the lock if it is still owned by the caller.
func (l *RedisLock) Release(ctx context.Context) error {
	if l.client == nil {
		return fmt.Errorf("astra/lock: redis client is nil")
	}
	released, err := redisLockReleaseScript.Run(ctx, l.client, []string{l.key}, l.token).Int64()
	if err != nil {
		return fmt.Errorf("astra/lock: %w", err)
	}
	if released == 0 {
		return ErrLockNotOwned
	}
	return nil
}

// Extend refreshes the lock TTL if it is still owned by the caller.
func (l *RedisLock) Extend(ctx context.Context, ttl time.Duration) error {
	if l.client == nil {
		return fmt.Errorf("astra/lock: redis client is nil")
	}
	if ttl <= 0 {
		return fmt.Errorf("astra/lock: ttl must be greater than zero")
	}

	extended, err := redisLockExtendScript.Run(ctx, l.client, []string{l.key}, l.token, ttl.Milliseconds()).Int64()
	if err != nil {
		return fmt.Errorf("astra/lock: %w", err)
	}
	if extended == 0 {
		return ErrLockNotOwned
	}
	return nil
}

func generateLockToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func normalizeLockPrefix(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = defaultRedisLockPrefix
	}
	if !strings.HasSuffix(trimmed, ":") {
		trimmed += ":"
	}
	return trimmed
}
