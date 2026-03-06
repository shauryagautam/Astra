package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

// SessionDriver defines the interface for a session driver.
type SessionDriver interface {
	Get(ctx context.Context, id string) (map[string]any, error)
	Set(ctx context.Context, id string, data map[string]any, ttl time.Duration) error
	Destroy(ctx context.Context, id string) error
}

// RedisSessionDriver implements SessionDriver using Redis.
type RedisSessionDriver struct {
	client *redis.Client
	prefix string
}

// NewRedisSessionDriver creates a new Redis session driver.
func NewRedisSessionDriver(client *redis.Client, prefix string) *RedisSessionDriver {
	if prefix == "" {
		prefix = "session:"
	}
	return &RedisSessionDriver{
		client: client,
		prefix: prefix,
	}
}

// Get retrieves a session by ID.
func (d *RedisSessionDriver) Get(ctx context.Context, id string) (map[string]any, error) {
	val, err := d.client.Get(ctx, d.prefix+id).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}

	var data map[string]any
	if err := sonic.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return data, nil
}

// Set stores a session.
func (d *RedisSessionDriver) Set(ctx context.Context, id string, data map[string]any, ttl time.Duration) error {
	bytes, err := sonic.Marshal(data)
	if err != nil {
		return err
	}
	return d.client.Set(ctx, d.prefix+id, bytes, ttl).Err()
}

// Destroy removes a session.
func (d *RedisSessionDriver) Destroy(ctx context.Context, id string) error {
	return d.client.Del(ctx, d.prefix+id).Err()
}
