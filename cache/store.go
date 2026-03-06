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

// MSet stores multiple values in the cache with the same expiration.
func (s *Store) MSet(ctx context.Context, items map[string]any, expiration time.Duration) error {
	pipe := s.client.Pipeline()
	for k, v := range items {
		bytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		pipe.Set(ctx, k, bytes, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// MGet retrieves multiple values from the cache.
func (s *Store) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	pipe := s.client.Pipeline()
	for _, k := range keys {
		pipe.Get(ctx, k)
	}
	cmds, _ := pipe.Exec(ctx)

	results := make(map[string][]byte)
	for i, cmd := range cmds {
		if getCmd, ok := cmd.(*redis.StringCmd); ok {
			bytes, err := getCmd.Bytes()
			if err == nil {
				results[keys[i]] = bytes
			}
		}
	}
	return results, nil
}

// Flush removes all keys from the cache (flushes the current DB).
func (s *Store) Flush(ctx context.Context) error {
	return s.client.FlushDB(ctx).Err()
}

// Remember retrieves an item from the cache, or executes the given function and stores the result.
func Remember[T any](ctx context.Context, s *Store, key string, expiration time.Duration, fn func() (T, error)) (T, error) {
	var val T
	err := s.Get(ctx, key, &val)
	if err == nil {
		return val, nil
	}

	val, err = fn()
	if err != nil {
		return val, err
	}

	err = s.Set(ctx, key, val, expiration)
	return val, err
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
