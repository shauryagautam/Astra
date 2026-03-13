package middleware

import (
	"fmt"
	"time"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/http"
)

// DashboardLogger tracks HTTP requests in the Dev Dashboard.
func DashboardLogger(app *core.App) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			start := time.Now()

			// Process request
			err := next(c)

			// Track in dashboard if exists
			if dash, ok := app.Get("dashboard").(*core.Dashboard); ok {
				msg := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
				dash.Track("http", msg, map[string]any{
					"method":   c.Request.Method,
					"path":     c.Request.URL.Path,
					"status":   c.Status(),
					"duration": time.Since(start).String(),
					"ip":       c.Request.RemoteAddr,
				})
			}

			return err
		}
	}
}
