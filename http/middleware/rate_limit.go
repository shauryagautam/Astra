package middleware

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"time"

	astrahttp "github.com/astraframework/astra/http"
	goredis "github.com/redis/go-redis/v9"
)

// IdentifierFunc resolves the rate-limit bucket key for a request.
type IdentifierFunc func(c *astrahttp.Context) string

// RateLimitOption configures Redis-backed rate limiting.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	identifier      IdentifierFunc
	keyPrefix       string
	apiKeyHeader    string
	trustedProxies  []netip.Prefix
	useIPIdentifier bool
	fallbackToByIP  bool
	algorithm       astrahttp.RateLimitAlgorithm
}

// ByIP buckets requests by client IP address.
func ByIP(c *astrahttp.Context) string {
	return requestRemoteIP(c)
}

// ByUser buckets requests by authenticated user ID.
func ByUser(c *astrahttp.Context) string {
	if claims := c.AuthUser(); claims != nil {
		return strings.TrimSpace(claims.UserID)
	}
	return ""
}

// ByAPIKey buckets requests by API key header.
func ByAPIKey(c *astrahttp.Context) string {
	return strings.TrimSpace(c.Header("X-API-Key"))
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
func WithAlgorithm(algo astrahttp.RateLimitAlgorithm) RateLimitOption {
	return func(cfg *rateLimitConfig) {
		cfg.algorithm = algo
	}
}

// RateLimit returns a Redis-backed sliding-window rate limiter middleware.
func RateLimit(client goredis.UniversalClient, limit int, window time.Duration, opts ...RateLimitOption) (astrahttp.MiddlewareFunc, error) {
	if client == nil {
		return nil, fmt.Errorf("astra: Redis is required for distributed rate limiting. Call app.WithRedis() before using RateLimit middleware")
	}

	cfg := rateLimitConfig{
		identifier:      ByIP,
		apiKeyHeader:    "X-API-Key", // #nosec G101
		useIPIdentifier: true,
		fallbackToByIP:  true,
		algorithm:       astrahttp.SlidingWindow,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	handler := func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			now := time.Now()
			identifier := resolveIdentifier(c, cfg)

			// Optimize key generation
			prefix := resolveKeyPrefix(c, cfg.keyPrefix)
			key := prefix + ":rl:" + identifier

			allowed, remaining, resetAt, err := astrahttp.RateLimitCheck(c, client, key, limit, window, cfg.algorithm)
			if err != nil {
				return err
			}

			c.SetHeader("X-RateLimit-Limit", strconv.Itoa(limit))
			c.SetHeader("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			c.SetHeader("X-RateLimit-Reset", strconv.FormatInt(time.UnixMilli(resetAt).Unix(), 10))

			if !allowed {
				retryAfter := int(math.Ceil(float64(resetAt-now.UnixMilli()) / 1000.0))
				if retryAfter < 1 {
					retryAfter = 1
				}
				c.SetHeader("Retry-After", strconv.Itoa(retryAfter))
				return c.JSON(map[string]any{
					"error":       "rate_limit_exceeded",
					"retry_after": retryAfter,
				}, http.StatusTooManyRequests)
			}

			return next(c)
		}
	}
	return handler, nil
}

// RateLimiter is an alias for RateLimit.
func RateLimiter(client goredis.UniversalClient, limit int, window time.Duration, opts ...RateLimitOption) (astrahttp.MiddlewareFunc, error) {
	return RateLimit(client, limit, window, opts...)
}

func resolveIdentifier(c *astrahttp.Context, cfg rateLimitConfig) string {
	if cfg.useIPIdentifier {
		return byTrustedIP(c, cfg.trustedProxies)
	}

	if isByAPIKey(cfg.identifier) {
		if key := strings.TrimSpace(c.Header(cfg.apiKeyHeader)); key != "" {
			return key
		}
	} else if cfg.identifier != nil {
		if value := strings.TrimSpace(cfg.identifier(c)); value != "" {
			return value
		}
	}

	if cfg.fallbackToByIP {
		return byTrustedIP(c, cfg.trustedProxies)
	}
	return "anonymous"
}

func resolveKeyPrefix(c *astrahttp.Context, override string) string {
	if override != "" {
		return override
	}
	if c.App != nil {
		if prefixed, ok := interface{}(c.App).(interface{ RedisKeyPrefix() string }); ok {
			return strings.Trim(prefixed.RedisKeyPrefix(), ": ")
		}
	}
	return "astra"
}

func byTrustedIP(c *astrahttp.Context, trustedProxies []netip.Prefix) string {
	remote := requestRemoteIP(c)
	remoteAddr, err := netip.ParseAddr(remote)
	if err != nil {
		return remote
	}
	if !isTrustedProxy(remoteAddr, trustedProxies) {
		return remote
	}

	if forwarded := strings.TrimSpace(c.Header("X-Forwarded-For")); forwarded != "" {
		for _, part := range strings.Split(forwarded, ",") {
			candidate := strings.TrimSpace(part)
			if addr, parseErr := netip.ParseAddr(candidate); parseErr == nil {
				return addr.String()
			}
		}
	}
	if realIP := strings.TrimSpace(c.Header("X-Real-IP")); realIP != "" {
		if addr, parseErr := netip.ParseAddr(realIP); parseErr == nil {
			return addr.String()
		}
	}
	return remote
}

func isTrustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
	if len(trusted) == 0 {
		return false
	}
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func requestRemoteIP(c *astrahttp.Context) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(c.Request.RemoteAddr)
}

func isByAPIKey(fn IdentifierFunc) bool {
	if fn == nil {
		return false
	}
	return reflect.ValueOf(fn).Pointer() == reflect.ValueOf(ByAPIKey).Pointer()
}
