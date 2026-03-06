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
	Client redis.UniversalClient
	config config.RedisConfig
}

// Name returns the service name.
func (r *Redis) Name() string {
	return "redis"
}

// Start initializes the Redis client.
func (r *Redis) Start(ctx context.Context) error {
	client, err := ConnectRedis(r.config)
	if err != nil {
		return err
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

// ConnectRedis creates a standalone Redis client (Universal support).
func ConnectRedis(cfg config.RedisConfig) (redis.UniversalClient, error) {
	var addrs []string
	var db int
	var password string

	if cfg.URL != "" {
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("cache: failed to parse Redis URL: %w", err)
		}
		addrs = []string{opt.Addr}
		db = opt.DB
		password = opt.Password
	} else {
		if cfg.UseSentinel {
			addrs = cfg.SentinelAddrs
		} else {
			addrs = []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)}
		}
		db = cfg.DB
		password = cfg.Password
	}

	opts := &redis.UniversalOptions{
		Addrs:        addrs,
		Password:     password,
		DB:           db,
		MaxRetries:   cfg.MaxRetries,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	if cfg.UseSentinel {
		opts.MasterName = cfg.SentinelMaster
	}

	client := redis.NewUniversalClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return client, nil
}
