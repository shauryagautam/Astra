package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/redis/go-redis/v9"
)

// Redis is the cache service holding the redis client.
type Redis struct {
	Client *redis.Client
	config config.RedisConfig
}

// Name returns the service name.
func (r *Redis) Name() string {
	return "redis"
}

// Start initializes the Redis client.
func (r *Redis) Start(ctx context.Context) error {
	var opts *redis.Options
	if r.config.URL != "" {
		opt, err := redis.ParseURL(r.config.URL)
		if err != nil {
			return fmt.Errorf("cache: failed to parse Redis URL: %w", err)
		}
		opts = opt
	} else {
		opts = &redis.Options{
			Addr:       fmt.Sprintf("%s:%d", r.config.Host, r.config.Port),
			Password:   r.config.Password,
			DB:         r.config.DB,
			MaxRetries: r.config.MaxRetries,
			PoolSize:   r.config.PoolSize,
		}
	}

	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cache: failed to ping Redis: %w", err)
	}

	r.Client = client
	return nil
}

// Stop closes the Redis client.
func (r *Redis) Stop(ctx context.Context) error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// New creates a new Redis service.
func New(cfg config.RedisConfig) *Redis {
	return &Redis{
		config: cfg,
	}
}

// ConnectRedis creates a standalone Redis client (useful for CLI and auto-loading).
func ConnectRedis(cfg config.RedisConfig) (*redis.Client, error) {
	var opts *redis.Options
	if cfg.URL != "" {
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("cache: failed to parse Redis URL: %w", err)
		}
		opts = opt
	} else {
		opts = &redis.Options{
			Addr:       fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password:   cfg.Password,
			DB:         cfg.DB,
			MaxRetries: cfg.MaxRetries,
			PoolSize:   cfg.PoolSize,
		}
	}

	// Upstash / production tuning
	if opts.PoolSize == 0 {
		opts.PoolSize = 10
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return client, nil
}
