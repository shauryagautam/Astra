package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store provides generic caching methods over Redis.
type Store struct {
	client *redis.Client
}

// NewStore creates a new cache store.
func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

// Set stores a value in the cache with an expiration.
func (s *Store) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, bytes, expiration).Err()
}

// Forever stores a value in the cache without an expiration.
func (s *Store) Forever(ctx context.Context, key string, value any) error {
	return s.Set(ctx, key, value, 0)
}

// Get retrieves a value from the cache.
func (s *Store) Get(ctx context.Context, key string, dest any) error {
	bytes, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil // Or return a specific ErrCacheMiss
		}
		return err
	}
	return json.Unmarshal(bytes, dest)
}

// GetString retrieves a string from the cache.
func (s *Store) GetString(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return "", err
	}
	return val, nil
}

// Forget removes a value from the cache.
func (s *Store) Forget(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// ForgetMany removes multiple keys from the cache.
func (s *Store) ForgetMany(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

// Increment increments an integer value in the cache.
func (s *Store) Increment(ctx context.Context, key string) (int64, error) {
	return s.client.Incr(ctx, key).Result()
}

// Decrement decrements an integer value in the cache.
func (s *Store) Decrement(ctx context.Context, key string) (int64, error) {
	return s.client.Decr(ctx, key).Result()
}

// Flush removes all keys from the cache (flushes the current DB).
func (s *Store) Flush(ctx context.Context) error {
	return s.client.FlushDB(ctx).Err()
}

// TaggedStore provides tagged caching methods over Redis.
type TaggedStore struct {
	*Store
	tags []string
}

// Tags creates a new TaggedStore with the given tags.
func (s *Store) Tags(names ...string) *TaggedStore {
	return &TaggedStore{
		Store: s,
		tags:  names,
	}
}

func (t *TaggedStore) tagKey(tag string) string {
	return "cache:tag:" + tag + ":keys"
}

// Set stores a value in the cache and associates it with the tags.
func (t *TaggedStore) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	if err := t.Store.Set(ctx, key, value, expiration); err != nil {
		return err
	}
	for _, tag := range t.tags {
		t.client.SAdd(ctx, t.tagKey(tag), key)
	}
	return nil
}

// Flush removes all keys associated with the tags.
func (t *TaggedStore) Flush(ctx context.Context) error {
	for _, tag := range t.tags {
		tk := t.tagKey(tag)
		keys, err := t.client.SMembers(ctx, tk).Result()
		if err != nil && err != redis.Nil {
			return err
		}
		if len(keys) > 0 {
			if err := t.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if err := t.client.Del(ctx, tk).Err(); err != nil {
			return err
		}
	}
	return nil
}
