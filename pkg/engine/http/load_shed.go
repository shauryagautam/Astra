package http

import (
	"math"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

)

type LoadShedConfig struct {
	// MinConcurrency is the floor below which the limit will never drop.
	MinConcurrency int64
	// MaxConcurrency is the ceiling (default: runtime.GOMAXPROCS*16).
	MaxConcurrency int64
	// LimitMultiplier is the AIMD decrease factor when overloaded (default: 0.9).
	LimitMultiplier float64
	// WindowDuration is how often the limit is recalculated (default: 200ms).
	WindowDuration time.Duration
	// QueueDepth is how many requests may wait above the limit before shedding.
	// 0 means shed immediately when the limit is reached.
	QueueDepth int64
	// BackgroundLatencyThreshold is the latency multiplier (relative to baseline) 
	// at which background traffic starts being shed. Default: 1.5.
	BackgroundLatencyThreshold float64
	// NormalLatencyThreshold is the latency multiplier at which normal 
	// traffic starts being shed. Default: 2.0.
	NormalLatencyThreshold float64
	// CriticalLatencyThreshold is the latency multiplier at which even 
	// critical traffic starts being shed. Default: 5.0.
	CriticalLatencyThreshold float64
	// RetryAfter is the value of the Retry-After header sent on 503s (seconds).
	RetryAfter int
}

type Priority string

const (
	PriorityCritical   Priority = "critical"
	PriorityNormal     Priority = "normal"
	PriorityBackground Priority = "background"
	PriorityAnalytics  Priority = "analytics"
)


func (c *LoadShedConfig) setDefaults() {
	procs := int64(runtime.GOMAXPROCS(0))
	if c.MinConcurrency <= 0 {
		c.MinConcurrency = procs
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = procs * 16
	}
	if c.LimitMultiplier <= 0 || c.LimitMultiplier >= 1 {
		c.LimitMultiplier = 0.9
	}
	if c.WindowDuration <= 0 {
		c.WindowDuration = 200 * time.Millisecond
	}
	if c.RetryAfter <= 0 {
		c.RetryAfter = 1
	}
	if c.BackgroundLatencyThreshold <= 0 {
		c.BackgroundLatencyThreshold = 1.5
	}
	if c.NormalLatencyThreshold <= 0 {
		c.NormalLatencyThreshold = 2.0
	}
	if c.CriticalLatencyThreshold <= 0 {
		c.CriticalLatencyThreshold = 5.0
	}
}


// loadSheddingState holds the mutable state for the adaptive limiter.
// Fields are 64-bit aligned to be safe for atomic.Load/Store on 32-bit platforms.
type loadSheddingState struct {
	inflight    atomic.Int64
	limit       atomic.Int64
	windowTotal atomic.Int64 // accumulated latency ns in current window
	windowCount atomic.Int64 // requests observed in current window
	lastReset   atomic.Int64 // unix nano of last window boundary
}

// AdaptiveLoadShedding returns an AIMD-based adaptive concurrency middleware.
func AdaptiveLoadShedding(cfg LoadShedConfig) MiddlewareFunc {
	cfg.setDefaults()

	state := &loadSheddingState{}
	state.limit.Store(cfg.MaxConcurrency)
	state.lastReset.Store(time.Now().UnixNano())

	// baseline latency in ns — the EMA baseline
	var baselineNs atomic.Int64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			priority := Priority(r.Header.Get("X-Astra-Priority"))
			if priority == "" {
				priority = PriorityNormal
			}

			current := state.inflight.Add(1)
			defer state.inflight.Add(-1)

			limit := state.limit.Load()
			baseline := baselineNs.Load()

			// 1. Hard concurrency limit check
			if current > limit+cfg.QueueDepth {
				shedRequest(w, cfg.RetryAfter, "concurrency-limit")
				return
			}

			// 2. Adaptive Latency-based Throttling
			if baseline > 0 {
				windowWait := state.windowCount.Load()
				if windowWait > 5 {
					avgSoFar := state.windowTotal.Load() / windowWait
					ratio := float64(avgSoFar) / float64(baseline)

					switch priority {
					case PriorityAnalytics, PriorityBackground:
						if ratio > cfg.BackgroundLatencyThreshold {
							shedRequest(w, cfg.RetryAfter, "latency-spike-background")
							return
						}
					case PriorityNormal:
						if ratio > cfg.NormalLatencyThreshold {
							shedRequest(w, cfg.RetryAfter, "latency-spike-normal")
							return
						}
					case PriorityCritical:
						if ratio > cfg.CriticalLatencyThreshold {
							shedRequest(w, cfg.RetryAfter, "latency-spike-critical")
							return
						}
					}
				}
			}

			start := time.Now()
			next.ServeHTTP(w, r)
			latency := time.Since(start).Nanoseconds()

			// Accumulate window stats
			state.windowTotal.Add(latency)
			count := state.windowCount.Add(1)

			// Window limit recalculation
			now := time.Now().UnixNano()
			windowNs := cfg.WindowDuration.Nanoseconds()
			lastReset := state.lastReset.Load()

			if now-lastReset >= windowNs && state.lastReset.CompareAndSwap(lastReset, now) {
				// Update EMA Baseline
				avgLatency := state.windowTotal.Swap(0) / max64(count, 1)
				state.windowCount.Store(0)

				if baseline == 0 {
					baselineNs.Store(avgLatency)
				} else {
					// Exponential Moving Average (EMA) Update
					newBaseline := (baseline*7 + avgLatency) / 8
					baselineNs.Store(newBaseline)

					currentLimit := state.limit.Load()
					if avgLatency <= int64(float64(baseline)*1.1) {
						newLimit := currentLimit + 1
						if newLimit > cfg.MaxConcurrency {
							newLimit = cfg.MaxConcurrency
						}
						state.limit.Store(newLimit)
					} else {
						newLimit := int64(math.Round(float64(currentLimit) * cfg.LimitMultiplier))
						if newLimit < cfg.MinConcurrency {
							newLimit = cfg.MinConcurrency
						}
						state.limit.Store(newLimit)
					}
				}
			}
		})
	}
}

func shedRequest(w http.ResponseWriter, retryAfter int, reason string) {
	w.Header().Set("Retry-After", itoa(retryAfter))
	w.Header().Set("X-Astra-Shed-Reason", reason)
	w.WriteHeader(http.StatusServiceUnavailable)
}


func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// itoa converts a small int to string without fmt import.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte(n%10) + '0'
		n /= 10
	}
	return string(buf[pos:])
}
