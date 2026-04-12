package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"syscall"
	"time"
)

// HealthStatus represents the health of a component.
type HealthStatus string

const (
	StatusOk      HealthStatus = "ok"
	StatusWarning HealthStatus = "warning"
	StatusError   HealthStatus = "error"
)

// HealthCheckFunc is a function that performs a health check.
type HealthCheckFunc func(ctx context.Context) error

// ComponentStatus holds the health information for a single component.
type ComponentStatus struct {
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Duration  string       `json:"duration,omitempty"`
}

// HealthReport is the full report returned by the HealthChecker.
type HealthReport struct {
	Status     HealthStatus               `json:"status"`
	Components map[string]ComponentStatus `json:"components"`
	Timestamp  time.Time                  `json:"timestamp"`
}

// HealthChecker manages and executes health checks.
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]HealthCheckFunc
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheckFunc),
	}
}

// Register registers a new health check.
func (h *HealthChecker) Register(name string, check HealthCheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// Report executes all registered health checks and returns a consolidated report.
func (h *HealthChecker) Report(ctx context.Context, depth string) any {
	h.mu.RLock()
	checks := make(map[string]HealthCheckFunc)
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	report := HealthReport{
		Status:     StatusOk,
		Components: make(map[string]ComponentStatus),
		Timestamp:  time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, check := range checks {
		wg.Add(1)
		go func(n string, c HealthCheckFunc) {
			defer wg.Done()

			start := time.Now()
			err := c(ctx)
			duration := time.Since(start)

			status := StatusOk
			msg := ""
			if err != nil {
				status = StatusError
				msg = err.Error()
			}

			mu.Lock()
			report.Components[n] = ComponentStatus{
				Status:    status,
				Message:   msg,
				Timestamp: time.Now(),
				Duration:  duration.String(),
			}
			if status == StatusError {
				report.Status = StatusError
			}
			mu.Unlock()
		}(name, check)
	}

	wg.Wait()

	return report
}

// DiskSpaceCheck returns a health check that fails if disk space is low.
func DiskSpaceCheck(path string, minFreeBytes uint64) HealthCheckFunc {
	return func(ctx context.Context) error {
		var stat syscall.Statfs_t
		err := syscall.Statfs(path, &stat)
		if err != nil {
			return fmt.Errorf("disk: failed to stat path %q: %w", path, err)
		}

		free := stat.Bavail * uint64(stat.Bsize)
		if free < minFreeBytes {
			return fmt.Errorf("disk: low space on %q: %d bytes free (min %d)", path, free, minFreeBytes)
		}
		return nil
	}
}

// LatencyCheck returns a health check that performs an HTTP GET and fails if latency is too high.
func LatencyCheck(url string, threshold time.Duration) HealthCheckFunc {
	return func(ctx context.Context) error {
		start := time.Now()

		// Create HTTP request with context timeout
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("latency: failed to create request: %w", err)
		}

		// Perform HTTP request
		client := &http.Client{
			Timeout: threshold,
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("latency: request failed: %w", err)
		}
		defer resp.Body.Close()

		duration := time.Since(start)
		if duration > threshold {
			return fmt.Errorf("latency: check took %v (limit %v)", duration, threshold)
		}

		return nil
	}
}
