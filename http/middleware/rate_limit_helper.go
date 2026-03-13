package middleware

import (
	"time"

	astrahttp "github.com/astraframework/astra/http"
	goredis "github.com/redis/go-redis/v9"
)

// SensitiveRateLimit returns a restrictive rate limiter for sensitive routes like Login or Register.
// By default, it allows 5 requests per minute per IP.
func SensitiveRateLimit(client goredis.UniversalClient, opts ...RateLimitOption) (astrahttp.MiddlewareFunc, error) {
	return RateLimit(client, 5, time.Minute, opts...)
}

// AuthRateLimit is a preset for general authentication-related routes.
// By default, it allows 10 requests per minute.
func AuthRateLimit(client goredis.UniversalClient, opts ...RateLimitOption) (astrahttp.MiddlewareFunc, error) {
	return RateLimit(client, 10, time.Minute, opts...)
}
