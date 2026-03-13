package http

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var requestSequence uint64

// RateLimitAlgorithm defines the strategy used for rate limiting.
type RateLimitAlgorithm string

const (
	SlidingWindow RateLimitAlgorithm = "sliding_window"
	TokenBucket   RateLimitAlgorithm = "token_bucket"
)

// RateLimitOption defines functional options for the RateLimit middleware.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	Limit       int
	Window      time.Duration
	KeyFunc     func(c *Context) string
	RedisClient goredis.UniversalClient
	Algorithm   RateLimitAlgorithm
}

// DefaultKeyFunc uses the request IP as the default rate limit key.
func DefaultKeyFunc(c *Context) string {
	return "ip:" + c.RealIP()
}

// RateLimit returns a middleware that limits requests based on the provided options.
// It uses Redis for distributed rate limiting if a Redis client is provided,
// otherwise it could fall back to an in-memory implementation (to be added if needed).
func RateLimit(limit int, window time.Duration, opts ...RateLimitOption) MiddlewareFunc {
	config := &rateLimitConfig{
		Limit:     limit,
		Window:    window,
		KeyFunc:   DefaultKeyFunc,
		Algorithm: SlidingWindow,
	}

	for _, opt := range opts {
		opt(config)
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			// Ensure we have a Redis client. If not provided via options, try to get from app container.
			client := config.RedisClient
			if client == nil && c.App != nil {
				if r := c.App.Get("redis"); r != nil {
					client = r.(goredis.UniversalClient)
				}
			}

			if client == nil {
				// If no redis client is available, we skip rate limiting for now
				// or we could implement an in-memory fallback.
				return next(c)
			}

			key := "ratelimit:" + string(config.Algorithm) + ":" + config.KeyFunc(c)
			allowed, remaining, resetAt, err := RateLimitCheck(c, client, key, config.Limit, config.Window, config.Algorithm)
			if err != nil {
				c.App.Logger.Error("rate limit check failed", "error", err)
				return next(c) // Fail open to avoid blocking users on infrastructure issues
			}

			// Set rate limit headers
			c.Writer.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", config.Limit))
			c.Writer.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			c.Writer.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt/1000))

			if !allowed {
				return NewHTTPError(http.StatusTooManyRequests, ErrCodeRateLimit, "Too many requests. Please try again later.")
			}

			return next(c)
		}
	}
}

// WithRedisClient sets a custom Redis client for the rate limiter.
func WithRedisClient(client goredis.UniversalClient) RateLimitOption {
	return func(c *rateLimitConfig) {
		c.RedisClient = client
	}
}

// WithKeyFunc sets a custom key generator for the rate limiter.
func WithKeyFunc(fn func(c *Context) string) RateLimitOption {
	return func(c *rateLimitConfig) {
		c.KeyFunc = fn
	}
}

// WithAlgorithm sets the rate limit tracking algorithm (SlidingWindow or TokenBucket).
func WithAlgorithm(algo RateLimitAlgorithm) RateLimitOption {
	return func(c *rateLimitConfig) {
		c.Algorithm = algo
	}
}

// RateLimitScript is the Lua script used for sliding window rate limiting in Redis.
var RateLimitScript = goredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local member = ARGV[4]
local minScore = now - window

redis.call("ZREMRANGEBYSCORE", key, "-inf", minScore)

local count = redis.call("ZCARD", key)
local allowed = 0

if count < limit then
	redis.call("ZADD", key, now, member)
	redis.call("PEXPIRE", key, window)
	count = count + 1
	allowed = 1
end

local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
local resetAt = now + window
if oldest[2] ~= nil then
	resetAt = tonumber(oldest[2]) + window
end

local remaining = limit - count
if remaining < 0 then
	remaining = 0
end

return {allowed, remaining, resetAt}
`)

// TokenBucketScript is the Lua script used for token bucket rate limiting in Redis.
var TokenBucketScript = goredis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local state = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(state[1])
local last_refill = tonumber(state[2])

if not tokens then
    tokens = capacity
    last_refill = now
else
    local elapsed = math.max(0, now - last_refill)
    local refill = elapsed * (capacity / window)
    tokens = math.min(capacity, tokens + refill)
    last_refill = now
end

local allowed = 0
if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
end

redis.call("HMSET", key, "tokens", tokens, "last_refill", last_refill)
redis.call("PEXPIRE", key, window)

local resetAt = now + window
if tokens < capacity then
	local time_to_fill = ((capacity - tokens) / (capacity / window))
	resetAt = now + math.ceil(time_to_fill)
end

return {allowed, math.floor(tokens), resetAt}
`)

// RateLimitCheck performs a manual rate limit check against Redis.
// It returns allowed (bool), remaining (int64), resetAt (unix milli), and error.
func RateLimitCheck(c *Context, client goredis.UniversalClient, key string, limit int, window time.Duration, algo RateLimitAlgorithm) (bool, int64, int64, error) {
	now := time.Now()

	var result interface{}
	var err error

	if algo == TokenBucket {
		result, err = TokenBucketScript.Run(
			c.Ctx(),
			client,
			[]string{key},
			limit,
			window.Milliseconds(),
			now.UnixMilli(),
			1, // request 1 token
		).Slice()
	} else {
		// Sliding Window is default
		member := fmt.Sprintf("%d:%d", now.UnixNano(), atomic.AddUint64(&requestSequence, 1))
		result, err = RateLimitScript.Run(
			c.Ctx(),
			client,
			[]string{key},
			limit,
			window.Milliseconds(),
			now.UnixMilli(),
			member,
		).Slice()
	}
	if err != nil {
		return false, 0, 0, fmt.Errorf("astra/rate_limit: %w", err)
	}

	return ParseRateLimitResult(result.([]interface{}))
}

// ParseRateLimitResult converts the Redis Lua script result into typed values.
func ParseRateLimitResult(result []interface{}) (bool, int64, int64, error) {
	if len(result) != 3 {
		return false, 0, 0, fmt.Errorf("astra/rate_limit: invalid redis response")
	}

	allowed, ok := result[0].(int64)
	if !ok {
		return false, 0, 0, fmt.Errorf("astra/rate_limit: invalid allowed value")
	}
	remaining, ok := result[1].(int64)
	if !ok {
		return false, 0, 0, fmt.Errorf("astra/rate_limit: invalid remaining value")
	}
	resetAt, ok := result[2].(int64)
	if !ok {
		return false, 0, 0, fmt.Errorf("astra/rate_limit: invalid reset value")
	}

	return allowed == 1, remaining, resetAt, nil
}
