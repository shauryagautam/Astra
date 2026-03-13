package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/astraframework/astra/cache"
	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/events"
)

// RedisProvider implements core.Provider for Redis services.
type RedisProvider struct {
	core.BaseProvider
}

// Register binds the Redis manager to the container without performing network operations.
func (p *RedisProvider) Register(a *core.App) error {
	// Only register if host or URL is provided
	if a.Config.Redis.Host == "" && a.Config.Redis.URL == "" {
		a.Logger.Debug("redis: host/url not configured, skipping provider registration")
		return nil
	}

	cfg := redisConfigFromAstra(a.Config.Redis)
	emitter, _ := a.Get("events").(*events.Emitter)
	manager := NewManager(cfg, emitter)
	a.Register("redis.manager", manager)

	// Register health check placeholder - will use the manager's client after Boot
	a.RegisterHealthCheck("redis", func(ctx context.Context) error {
		client, err := manager.Client()
		if err != nil {
			return err
		}
		return HealthCheck(ctx, client.UniversalClient)
	})

	return nil
}

// Boot connects to Redis, pings, and initializes Redis-backed services.
func (p *RedisProvider) Boot(a *core.App) error {
	svc := a.Get("redis.manager")
	if svc == nil {
		return nil
	}
	manager, ok := svc.(*Manager)
	if !ok {
		return fmt.Errorf("redis.Boot: registered manager is not of type *redis.Manager")
	}

	// Step 2: Boot connects and pings
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := manager.Connect(ctx); err != nil {
		return fmt.Errorf("redis.Boot: failed to connect to redis: %w", err)
	}

	client, err := manager.Client()
	if err != nil {
		return fmt.Errorf("redis.Boot: %w", err)
	}

	// Bind the underlying goredis client for convenience
	redisClient := client.UniversalClient
	a.Register("redis", redisClient)

	// Initialize Redis-backed services (cache, locker)
	if a.Get("cache") == nil {
		a.Register("cache", cache.NewRedisStore(redisClient, "astra:cache:"))
	}
	if a.Get("locker") == nil {
		a.Register("locker", cache.NewRedisLocker(redisClient, "astra:lock:"))
	}

	// Fallback: If NO cache was registered by any provider, use in-memory.
	if a.Get("cache") == nil {
		a.Logger.Info("cache: no store registered, failing back to in-memory store")
		a.Register("cache", cache.NewMemoryStore())
	}

	return nil
}

// Shutdown gracefully closes all Redis connections.
func (p *RedisProvider) Shutdown(ctx context.Context, a *core.App) error {
	svc := a.Get("redis.manager")
	if svc == nil {
		return nil
	}
	if manager, ok := svc.(*Manager); ok {
		return manager.Close(ctx)
	}
	return nil
}

func redisConfigFromAstra(cfg config.RedisConfig) config.RedisConfig {
	return cfg
}
