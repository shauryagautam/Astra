package http

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var requestSequence uint64

type RateLimitAlgorithm int

const (
	SlidingWindow RateLimitAlgorithm = iota
	TokenBucket
)

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
func RateLimitCheck(ctx context.Context, client goredis.UniversalClient, key string, limit int, window time.Duration, algo RateLimitAlgorithm) (bool, int64, int64, error) {
	now := time.Now()

	var result interface{}
	var err error

	if algo == TokenBucket {
		result, err = TokenBucketScript.Run(
			ctx,
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
			ctx,
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

// IdentifierFunc resolves the rate-limit bucket key for a request.
type IdentifierFunc func(r *http.Request) string

// RateLimitOption configures Redis-backed rate limiting.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	identifier      IdentifierFunc
	keyPrefix       string
	apiKeyHeader    string
	trustedProxies  []netip.Prefix
	useIPIdentifier bool
	fallbackToByIP  bool
	algorithm       RateLimitAlgorithm
	ipSpoofingProtection bool
	maxProxyDepth       int
	validateIPHeaders   bool
}

// ByIP buckets requests by client IP address.
func ByIP(r *http.Request) string {
	return requestRemoteIP(r)
}

// ByUser buckets requests by authenticated user ID.
func ByUser(r *http.Request) string {
	c := FromRequest(r)
	if c == nil {
		return ""
	}
	if claims := c.AuthUser(); claims != nil {
		return strings.TrimSpace(claims.UserID)
	}
	return ""
}

// ByAPIKey buckets requests by API key header.
func ByAPIKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

// WithIdentifier sets the request bucket resolver.
func WithIdentifier(fn IdentifierFunc) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.identifier = fn
		cfg.useIPIdentifier = fn == nil || reflect.ValueOf(fn).Pointer() == reflect.ValueOf(ByIP).Pointer()
	}
}

// WithTrustedProxies configures proxies allowed to supply forwarded IP headers.
func WithTrustedProxies(values ...string) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.trustedProxies = cfg.trustedProxies[:0]
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if strings.Contains(value, "/") {
				if prefix, err := netip.ParsePrefix(value); err == nil {
					cfg.trustedProxies = append(cfg.trustedProxies, prefix)
				}
				continue
			}
			if addr, err := netip.ParseAddr(value); err == nil {
				bits := 128
				if addr.Is4() {
					bits = 32
				}
				cfg.trustedProxies = append(cfg.trustedProxies, netip.PrefixFrom(addr, bits))
			}
		}
	}
}

// WithAPIKeyHeader changes the header used by ByAPIKey.
func WithAPIKeyHeader(header string) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		header = strings.TrimSpace(header)
		if header != "" {
			cfg.apiKeyHeader = header
		}
	}
}

// WithKeyPrefix overrides the Redis key namespace prefix used by rate limiting.
func WithKeyPrefix(prefix string) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.keyPrefix = strings.Trim(prefix, ": ")
	}
}

// WithAlgorithm sets the rate limiting algorithm to use (SlidingWindow or TokenBucket).
func WithAlgorithm(algo RateLimitAlgorithm) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.algorithm = algo
	}
}

// WithIPSpoofingProtection enables or disables IP spoofing protection.
func WithIPSpoofingProtection(enabled bool) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.ipSpoofingProtection = enabled
	}
}

// WithMaxProxyDepth sets the maximum number of proxy hops to trust in X-Forwarded-For headers.
func WithMaxProxyDepth(depth int) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.maxProxyDepth = depth
	}
}

// WithIPHeaderValidation enables or disables validation of IP headers.
func WithIPHeaderValidation(enabled bool) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.validateIPHeaders = enabled
	}
}

// RateLimit returns a standard Redis-backed rate limiter middleware.
func RateLimit(client goredis.UniversalClient, limit int, window time.Duration, opts ...RateLimitOption) (MiddlewareFunc, error) {
	if client == nil {
		return nil, fmt.Errorf("astra: Redis is required for distributed rate limiting")
	}

	cfg := rateLimitConfig{
		identifier:           ByIP,
		apiKeyHeader:         "X-API-Key",
		keyPrefix:            "astra",
		useIPIdentifier:      true,
		fallbackToByIP:       true,
		algorithm:            SlidingWindow,
		ipSpoofingProtection: true,
		maxProxyDepth:        5,
		validateIPHeaders:    true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := resolveIdentifier(r, cfg)
			
			prefix := strings.Trim(cfg.keyPrefix, ": ")
			key := prefix + ":rl:" + identifier

			allowed, remaining, resetAt, err := RateLimitCheck(r.Context(), client, key, limit, window, cfg.algorithm)
			if err != nil {
				c := FromRequest(r)
				if c != nil {
					c.InternalError(err.Error())
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.UnixMilli(resetAt).Unix(), 10))

			if !allowed {
				retryAfter := int(math.Ceil(float64(resetAt-time.Now().UnixMilli()) / 1000.0))
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				
				c := FromRequest(r)
				if c != nil {
					_ = c.JSON(map[string]any{
						"error":       "rate_limit_exceeded",
						"retry_after": retryAfter,
					}, http.StatusTooManyRequests)
				} else {
					w.WriteHeader(http.StatusTooManyRequests)
					fmt.Fprintf(w, `{"error":"rate_limit_exceeded","retry_after":%d}`, retryAfter)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}

// RateLimiter is an alias for RateLimit.
func RateLimiter(client goredis.UniversalClient, limit int, window time.Duration, opts ...RateLimitOption) (MiddlewareFunc, error) {
	return RateLimit(client, limit, window, opts...)
}

func resolveIdentifier(r *http.Request, cfg rateLimitConfig) string {
	if cfg.useIPIdentifier {
		return byTrustedIP(r, cfg.trustedProxies)
	}

	if isByAPIKey(cfg.identifier) {
		if key := strings.TrimSpace(r.Header.Get(cfg.apiKeyHeader)); key != "" {
			return key
		}
	} else if cfg.identifier != nil {
		if value := strings.TrimSpace(cfg.identifier(r)); value != "" {
			return value
		}
	}

	if cfg.fallbackToByIP {
		return byTrustedIP(r, cfg.trustedProxies)
	}
	return "anonymous"
}

func byTrustedIP(r *http.Request, trustedProxies []netip.Prefix) string {
	return GetClientIP(r, trustedProxies)
}

func isByAPIKey(fn IdentifierFunc) bool {
	if fn == nil {
		return false
	}
	return reflect.ValueOf(fn).Pointer() == reflect.ValueOf(ByAPIKey).Pointer()
}
