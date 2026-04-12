package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/cache"
)

// RedisProvider implements engine.Provider for Redis services.
type RedisProvider struct {
	engine.BaseProvider
	manager *Manager
}

// Register binds the Redis manager to the container without performing network operations.
func (p *RedisProvider) Register(a *engine.App) error {
	// Only register if host or URL is provided
	if a.Config().Redis.Host == "" && a.Config().Redis.URL == "" {
		a.Logger().Debug("redis: host/url not configured, skipping provider registration")
		return nil
	}

	cfg := redisConfigFromAstra(a.Config().Redis)
	p.manager = NewManager(cfg, nil)

	// Register health check placeholder
	a.RegisterHealthCheck("redis", engine.HealthCheckFunc(func(ctx context.Context) error {
		if p.manager == nil {
			return fmt.Errorf("redis manager not initialized")
		}
		_, err := p.manager.Client()
		return err
	}))

	return nil
}

// Boot connects to Redis, pings, and initializes Redis-backed services.
func (p *RedisProvider) Boot(a *engine.App) error {
	if p.manager == nil {
		return nil
	}
	manager := p.manager

	// Boot connects and pings
	ctx, cancel := context.WithTimeout(a.BaseContext(), 10*time.Second)
	defer cancel()
	if err := manager.Connect(ctx); err != nil {
		return fmt.Errorf("redis.Boot: failed to connect to redis: %w", err)
	}

	client, err := manager.ConnectAndGet(ctx)
	if err != nil {
		return fmt.Errorf("redis.Boot: %w", err)
	}

	// Initialize Redis-backed cache
	store := cache.NewRedisStore(client, "astra:cache:")
	// Ideally, this store should be injected into whoever needs it via Wire
	// instead of being used here. But for now, we'll keep it as a side effect
	// until the entire framework is fully Wire-compliant.
	_ = store

	return nil
}

// Shutdown gracefully closes all Redis connections.
func (p *RedisProvider) Shutdown(ctx context.Context, a *engine.App) error {
	if p.manager == nil {
		return nil
	}
	return p.manager.Close(ctx)
}

func redisConfigFromAstra(cfg config.RedisConfig) config.RedisConfig {
	return cfg
}

// ConnectAndGet is a helper to connect and return the client.
func (m *Manager) ConnectAndGet(ctx context.Context) (redis.UniversalClient, error) {
	if err := m.Connect(ctx); err != nil {
		return nil, err
	}
	c, err := m.Client()
	if err != nil {
		return nil, err
	}
	return c.UniversalClient, nil
}
