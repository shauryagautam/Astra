package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/shauryagautam/Astra/pkg/engine/telemetry"
)

// DashboardLogger tracks HTTP requests in the Dev Dashboard.
// It is fully decoupled from the kernel and accepts an explicit Dashboard dependency.
func DashboardLogger(dash *telemetry.Dashboard) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if dash == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Use responseWriter to capture status
			rw := &responseWriter{ResponseWriter: w}

			// Process request
			next.ServeHTTP(rw, r)

			// Track in dashboard
			status := rw.Status()
			msg := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			dash.Track("http", msg, map[string]any{
				"method":   r.Method,
				"path":     r.URL.Path,
				"status":   status,
				"duration": time.Since(start).String(),
				"ip":       r.RemoteAddr,
			})
		})
	}
}
