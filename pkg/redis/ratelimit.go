package redis

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RateLimiter provides rate limiting functionality.
type RateLimiter struct {
	client *Client
}

// NewRateLimiter creates a new RateLimiter instance.
func (c *Client) NewRateLimiter() *RateLimiter {
	return &RateLimiter{client: c}
}

// Allow checks if an action should be allowed based on a rate limit.
// It uses a sliding window algorithm implemented via a Redis Lua script.
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	script := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		-- Remove old entries
		redis.call("zremrangebyscore", key, 0, now - window)
		
		-- Count current entries
		local current = redis.call("zcard", key)
		
		if current < limit then
			-- Add new entry
			redis.call("zadd", key, now, now)
			redis.call("pexpire", key, window)
			return {1, limit - current - 1}
		else
			return {0, 0}
		end
	`

	// Sanitize key to prevent injection
	sanitizedKey := sanitizeRedisKey(key)
	now := time.Now().UnixMilli()
	res, err := rl.client.Eval(ctx, script, []string{"ratelimit:" + sanitizedKey}, limit, window.Milliseconds(), now).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis: rate limit check failed: %w", err)
	}

	parts := res.([]any)
	allowed := parts[0].(int64) == 1
	remaining := int(parts[1].(int64))

	return allowed, remaining, nil
}

// sanitizeRedisKey removes potentially dangerous characters from Redis keys
func sanitizeRedisKey(key string) string {
	// Remove whitespace, newlines, and special characters that could be used for injection
	key = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return '_'
		}
		return r
	}, key)
	return key
}
