package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

// responseWriter captures the HTTP status code for logging purposes.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Status() int {
	if rw.status == 0 {
		return http.StatusOK
	}
	return rw.status
}


// Recover returns a middleware that recovers from panics and returns a 500 error.
func Recover(logger *slog.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if logger != nil {
						logger.Error("panic recovered",
							"error", err,
							"stack", string(debug.Stack()),
						)
					}
					// Send standard 500
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "Internal Server Error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID returns a middleware that injects a unique request ID into the context and response headers.
func RequestID() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.NewString()
			}

			// Store in request context
			ctx := context.WithValue(r.Context(), "request_id", id)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r)
		})
	}
}

// Logger returns a middleware that logs incoming requests.
func Logger(logger *slog.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			
			// Use our responseWriter to capture status
			rw := &responseWriter{ResponseWriter: w}
			
			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			status := rw.Status()
			
			msg := fmt.Sprintf("%d %s %s", status, r.Method, r.URL.Path)
			
			attrs := []any{
				slog.Int("status", status),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Duration("duration", duration),
				slog.String("ip", r.RemoteAddr),
			}

			if reqID := r.Context().Value("request_id"); reqID != nil {
				attrs = append(attrs, slog.Any("request_id", reqID))
			}

			if status >= 500 {
				logger.Error(msg, attrs...)
			} else {
				logger.Info(msg, attrs...)
			}
		})
	}
}
