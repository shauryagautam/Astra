package providers

import (
	"fmt"
	"time"

	redis "github.com/shaurya/astra/app/Redis"
	"github.com/shaurya/astra/contracts"
)

// RedisProvider registers Redis services into the container.
// Mirrors Astra's RedisProvider.
type RedisProvider struct {
	BaseProvider
}

// NewRedisProvider creates a new RedisProvider.
func NewRedisProvider(app contracts.ApplicationContract) *RedisProvider {
	return &RedisProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds Redis manager, cache, rate limiter, and session store.
func (p *RedisProvider) Register() error {
	// Register the Redis manager
	p.App.Singleton("Astra/Redis", func(c contracts.ContainerContract) (any, error) {
		env := c.Use("Env").(*EnvManager)

		config := redis.ManagerConfig{
			Default: env.Get("REDIS_CONNECTION", "local"),
			Connections: map[string]redis.ConnectionConfig{
				"local": {
					Host:     env.Get("REDIS_HOST", "127.0.0.1"),
					Port:     parseIntOr(env.Get("REDIS_PORT", "6379"), 6379),
					Password: env.Get("REDIS_PASSWORD", ""),
					DB:       parseIntOr(env.Get("REDIS_DB", "0"), 0),
				},
			},
		}

		return redis.NewManager(config), nil
	})
	p.App.Alias("Redis", "Astra/Redis")

	// Register the Cache backed by Redis
	p.App.Singleton("Astra/Cache", func(c contracts.ContainerContract) (any, error) {
		redisMgr := c.Use("Redis").(*redis.Manager)
		conn := redisMgr.Default()
		return redis.NewCache(conn, "astra:cache:"), nil
	})
	p.App.Alias("Cache", "Astra/Cache")

	// Register the Rate Limiter
	p.App.Singleton("Astra/RateLimiter", func(c contracts.ContainerContract) (any, error) {
		redisMgr := c.Use("Redis").(*redis.Manager)
		conn := redisMgr.Default()
		return redis.NewRateLimiter(conn, "astra:ratelimit:"), nil
	})
	p.App.Alias("RateLimiter", "Astra/RateLimiter")

	// Register Redis-backed Session Store
	p.App.Singleton("Astra/SessionStore", func(c contracts.ContainerContract) (any, error) {
		redisMgr := c.Use("Redis").(*redis.Manager)
		conn := redisMgr.Default()
		ttl := 24 * time.Hour
		return redis.NewSessionStore(conn, ttl), nil
	})
	p.App.Alias("SessionStore", "Astra/SessionStore")

	// Register Redis-backed Token Store (for OAT auth)
	p.App.Singleton("Astra/RedisTokenStore", func(c contracts.ContainerContract) (any, error) {
		redisMgr := c.Use("Redis").(*redis.Manager)
		conn := redisMgr.Default()
		return redis.NewTokenStore(conn), nil
	})
	p.App.Alias("RedisTokenStore", "Astra/RedisTokenStore")

	return nil
}

// Shutdown closes all Redis connections.
func (p *RedisProvider) Shutdown() error {
	if p.App.HasBinding("Redis") {
		mgr, err := p.App.Make("Redis")
		if err == nil {
			if redisMgr, ok := mgr.(*redis.Manager); ok {
				return redisMgr.Quit()
			}
		}
	}
	return nil
}

func parseIntOr(s string, defaultVal int) int {
	var val int
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return defaultVal
	}
	return val
}
