package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const defaultRedisCachePrefix = "astra:cache:"

// RedisStore is a Redis-backed implementation of Store.
type RedisStore struct {
	client    goredis.UniversalClient
	keyPrefix string
}

// NewRedisStore creates a new Redis-backed cache store.
func NewRedisStore(client goredis.UniversalClient, keyPrefix string) *RedisStore {
	return &RedisStore{
		client:    client,
		keyPrefix: normalizeRedisKeyPrefix(keyPrefix, defaultRedisCachePrefix),
	}
}

// Get retrieves a value from Redis.
func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("astra/cache: redis client is nil")
	}
	value, err := s.client.Get(ctx, s.key(key)).Result()
	if err == nil {
		return value, nil
	}
	if errors.Is(err, goredis.Nil) {
		return "", ErrCacheMiss
	}
	return "", fmt.Errorf("astra/cache: %w", err)
}

// Set stores a value in Redis.
func (s *RedisStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if s.client == nil {
		return fmt.Errorf("astra/cache: redis client is nil")
	}
	if err := s.client.Set(ctx, s.key(key), value, ttl).Err(); err != nil {
		return fmt.Errorf("astra/cache: %w", err)
	}
	return nil
}

// Delete removes a value from Redis.
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if s.client == nil {
		return fmt.Errorf("astra/cache: redis client is nil")
	}
	if err := s.client.Del(ctx, s.key(key)).Err(); err != nil {
		return fmt.Errorf("astra/cache: %w", err)
	}
	return nil
}

// Has reports whether a value exists in Redis.
func (s *RedisStore) Has(ctx context.Context, key string) (bool, error) {
	if s.client == nil {
		return false, fmt.Errorf("astra/cache: redis client is nil")
	}
	count, err := s.client.Exists(ctx, s.key(key)).Result()
	if err != nil {
		return false, fmt.Errorf("astra/cache: %w", err)
	}
	return count > 0, nil
}

// Flush deletes only keys owned by this store prefix.
func (s *RedisStore) Flush(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("astra/cache: redis client is nil")
	}
	var cursor uint64
	pattern := s.keyPrefix + "*"

	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("astra/cache: %w", err)
		}

		if len(keys) > 0 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("astra/cache: %w", err)
			}
		}

		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}

// Remember returns a cached value or computes, stores, and returns it.
func (s *RedisStore) Remember(ctx context.Context, key string, ttl time.Duration, fn func() (string, error)) (string, error) {
	value, err := s.Get(ctx, key)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		return "", err
	}

	value, err = fn()
	if err != nil {
		return "", err
	}

	if err := s.Set(ctx, key, value, ttl); err != nil {
		return "", err
	}
	return value, nil
}

// GetMany fetches many keys in a single pipeline round trip.
func (s *RedisStore) GetMany(ctx context.Context, keys []string) (map[string]string, error) {
	if s.client == nil {
		return nil, fmt.Errorf("astra/cache: redis client is nil")
	}
	results := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return results, nil
	}

	cmds := make(map[string]*goredis.StringCmd, len(keys))
	_, err := s.client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		for _, key := range keys {
			cmds[key] = pipe.Get(ctx, s.key(key))
		}
		return nil
	})
	if err != nil && !errors.Is(err, goredis.Nil) {
		return nil, fmt.Errorf("astra/cache: %w", err)
	}

	for key, cmd := range cmds {
		value, cmdErr := cmd.Result()
		switch {
		case cmdErr == nil:
			results[key] = value
		case errors.Is(cmdErr, goredis.Nil):
			continue
		default:
			return nil, fmt.Errorf("astra/cache: %w", cmdErr)
		}
	}

	return results, nil
}

// SetMany stores many values in a single pipeline round trip.
func (s *RedisStore) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	if s.client == nil {
		return fmt.Errorf("astra/cache: redis client is nil")
	}
	if len(items) == 0 {
		return nil
	}

	_, err := s.client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		for key, value := range items {
			pipe.Set(ctx, s.key(key), value, ttl)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("astra/cache: %w", err)
	}
	return nil
}

func (s *RedisStore) key(key string) string {
	return s.keyPrefix + key
}

func normalizeRedisKeyPrefix(prefix string, fallback string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = fallback
	}
	if !strings.HasSuffix(trimmed, ":") {
		trimmed += ":"
	}
	return trimmed
}
