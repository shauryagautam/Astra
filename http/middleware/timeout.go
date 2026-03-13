package middleware

import (
	"context"
	"time"

	astrahttp "github.com/astraframework/astra/http"
)

// Timeout returns a middleware that cancels the request context after a given duration.
func Timeout(timeout time.Duration) astrahttp.MiddlewareFunc {
	return func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
			defer cancel()

			c.Request = c.Request.WithContext(ctx)

			// Create a channel to catch the result of the next handler
			done := make(chan error, 1)
			go func() {
				done <- next(c)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return astrahttp.ErrInternal.WithMessage("Request timed out")
				}
				return ctx.Err()
			}
		}
	}
}
