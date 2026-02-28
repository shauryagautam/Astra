package auth

import (
	"context"
	"time"

	"github.com/shaurya/adonis/contracts"
)

// RedisBlacklist implements the BlacklistContract using Redis.
type RedisBlacklist struct {
	redis  contracts.RedisConnectionContract
	prefix string
}

// NewRedisBlacklist creates a new Redis-backed blacklist.
func NewRedisBlacklist(redis contracts.RedisConnectionContract) *RedisBlacklist {
	return &RedisBlacklist{
		redis:  redis,
		prefix: "auth:blacklist:",
	}
}

func (b *RedisBlacklist) Add(ctx context.Context, token string, expiration time.Duration) error {
	return b.redis.Set(ctx, b.prefix+token, "1", expiration)
}

func (b *RedisBlacklist) Has(ctx context.Context, token string) (bool, error) {
	return b.redis.Exists(ctx, b.prefix+token)
}

var _ contracts.BlacklistContract = (*RedisBlacklist)(nil)
