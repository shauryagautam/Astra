package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// UltraFastLogger provides high-performance request logging.
// Accepts an explicit logger to remain decoupled from the kernel App.
func UltraFastLogger(logger *slog.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Simple response writer wrapper for status
			rw := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.Status(),
				"duration", duration,
			)
		})
	}
}

// UltraFastRecover provides high-performance panic recovery.
func UltraFastRecover(logger *slog.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if logger != nil {
						logger.Error("panic recovered", "panic", err)
					}
					// Only write if we haven't written yet (heuristic: status is 0)
					if rw, ok := w.(*responseWriter); ok {
						if rw.Status() == 0 {
							rw.WriteHeader(http.StatusInternalServerError)
						}
					} else {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// UltraFastRequestID provides high-performance request ID generation
func UltraFastRequestID() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = simpleRequestID()
			}
			w.Header().Set("X-Request-ID", reqID)
			next.ServeHTTP(w, r)
		})
	}
}

var requestCounter int64

func simpleRequestID() string {
	count := atomic.AddInt64(&requestCounter, 1)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), count)
}
