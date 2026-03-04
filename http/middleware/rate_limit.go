package middleware

import (
	"fmt"
	"time"

	"github.com/astraframework/astra/http"
	"github.com/redis/go-redis/v9"
)

// RateLimiter returns a middleware that limits requests using a sliding window.
func RateLimiter(client *redis.Client, max int, window time.Duration) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			ip := c.ClientIP()
			key := fmt.Sprintf("rate_limit:%s", ip)
			now := time.Now().UnixMilli()

			ctx := c.Ctx()
			
			// Remove old entries
			client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", now-window.Milliseconds()))
			
			// Count current entries
			count, err := client.ZCard(ctx, key).Result()
			if err != nil {
				return err
			}

			if int(count) >= max {
				return http.NewHTTPError(429, "RATE_LIMIT_EXCEEDED", "Too many requests")
			}

			// Add new entry
			client.ZAdd(ctx, key, redis.Z{
				Score:  float64(now),
				Member: now, // could be UUID, using timestamp for simplicity
			})
			client.Expire(ctx, key, window)

			return next(c)
		}
	}
}
