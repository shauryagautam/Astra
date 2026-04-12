// Package middleware provides chaos engineering primitives for Astra.
//
// Chaos middleware injects configurable faults (latency spikes, random errors,
// connection resets) to test distributed system resilience under adversarial
// conditions. It must ONLY be enabled in non-production environments.
package http

import (
	"math/rand/v2"
	"net/http"
	"os"
	"time"

)

// ChaosConfig controls the fault injection behaviour.
type ChaosConfig struct {
	// Enabled switches chaos mode on. If false, the middleware is a no-op.
	// Forced to false when APP_ENV == "production".
	Enabled bool

	// LatencyProb is the probability [0,1] that a request gets an injected delay.
	LatencyProb float64
	// MinDelay / MaxDelay define the injected latency range.
	MinDelay time.Duration
	MaxDelay time.Duration

	// ErrorProb is the probability [0,1] that a request is immediately failed
	// with a 500 Internal Server Error.
	ErrorProb float64

	// TimeoutProb is the probability [0,1] that the handler context is cancelled
	// before calling next, simulating an upstream timeout / DB hang.
	TimeoutProb float64

	// ExcludePaths contains URL path prefixes that are immune to fault injection
	// (useful to protect health-check and metrics endpoints).
	ExcludePaths []string
}

// Chaos returns a middleware that injects faults according to cfg.
// It is automatically disabled in production regardless of cfg.Enabled.
//
//	r.Use(middleware.Chaos(middleware.ChaosConfig{
//	    Enabled:     true,
//	    LatencyProb: 0.05,
//	    MinDelay:    50 * time.Millisecond,
//	    MaxDelay:    500 * time.Millisecond,
//	    ErrorProb:   0.01,
//	}))
func Chaos(cfg ChaosConfig) MiddlewareFunc {
	// Hard safety guard — never inject faults in production.
	if os.Getenv("APP_ENV") == "production" || !cfg.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	if cfg.MinDelay <= 0 {
		cfg.MinDelay = 10 * time.Millisecond
	}
	if cfg.MaxDelay <= cfg.MinDelay {
		cfg.MaxDelay = cfg.MinDelay * 10
	}

	delayRange := int64(cfg.MaxDelay - cfg.MinDelay)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			for _, prefix := range cfg.ExcludePaths {
				if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 1. Latency injection
			if cfg.LatencyProb > 0 && rand.Float64() < cfg.LatencyProb {
				delay := cfg.MinDelay
				if delayRange > 0 {
					delay += time.Duration(rand.Int64N(delayRange))
				}
				select {
				case <-time.After(delay):
				case <-r.Context().Done():
					return
				}
			}

			// 2. Error injection
			if cfg.ErrorProb > 0 && rand.Float64() < cfg.ErrorProb {
				w.Header().Set("X-Chaos-Fault", "error")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"chaos: injected fault"}`))
				return
			}

			// 3. Timeout injection
			if cfg.TimeoutProb > 0 && rand.Float64() < cfg.TimeoutProb {
				w.Header().Set("X-Chaos-Fault", "timeout")
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
