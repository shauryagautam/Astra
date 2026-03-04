package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/astraframework/astra/http"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthStatus represents the status of a single health check.
type HealthStatus struct {
	Status    string `json:"status"`          // "ok" or "error"
	LatencyMs int64  `json:"latency_ms"`      // round-trip latency in milliseconds
	Error     string `json:"error,omitempty"` // error message if status is "error"
}

// HealthReport is the full health check response.
type HealthReport struct {
	Status  string                  `json:"status"` // overall: "healthy" or "degraded"
	Version string                  `json:"version"`
	Uptime  string                  `json:"uptime"`
	Checks  map[string]HealthStatus `json:"checks"`
}

// HealthChecker performs deep health checks against all registered dependencies.
type HealthChecker struct {
	db      *pgxpool.Pool
	redis   *redis.Client
	version string
	startAt time.Time
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(db *pgxpool.Pool, rdb *redis.Client, version string) *HealthChecker {
	return &HealthChecker{
		db:      db,
		redis:   rdb,
		version: version,
		startAt: time.Now(),
	}
}

// Check performs all registered health checks and returns a combined report.
func (h *HealthChecker) Check(ctx *http.Context) error {
	report := h.report(ctx.Ctx())
	status := 200
	if report.Status != "healthy" {
		status = 503
	}
	return ctx.JSON(report, status)
}

// Ready returns 200 OK only if all critical checks pass.
func (h *HealthChecker) Ready(ctx *http.Context) error {
	report := h.report(ctx.Ctx())
	if report.Status != "healthy" {
		return ctx.JSON(report, 503)
	}
	return ctx.JSON(map[string]string{"status": "ready"})
}

func (h *HealthChecker) report(ctx context.Context) HealthReport {
	checks := make(map[string]HealthStatus)
	overall := "healthy"

	if h.db != nil {
		checks["database"] = pingDB(ctx, h.db)
		if checks["database"].Status != "ok" {
			overall = "degraded"
		}
	}

	if h.redis != nil {
		checks["redis"] = pingRedis(ctx, h.redis)
		if checks["redis"].Status != "ok" {
			overall = "degraded"
		}
	}

	return HealthReport{
		Status:  overall,
		Version: h.version,
		Uptime:  formatUptime(time.Since(h.startAt)),
		Checks:  checks,
	}
}

func pingDB(ctx context.Context, pool *pgxpool.Pool) HealthStatus {
	start := time.Now()
	if err := pool.Ping(ctx); err != nil {
		return HealthStatus{Status: "error", LatencyMs: msSince(start), Error: err.Error()}
	}
	return HealthStatus{Status: "ok", LatencyMs: msSince(start)}
}

func pingRedis(ctx context.Context, rdb *redis.Client) HealthStatus {
	start := time.Now()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return HealthStatus{Status: "error", LatencyMs: msSince(start), Error: err.Error()}
	}
	return HealthStatus{Status: "ok", LatencyMs: msSince(start)}
}

func msSince(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}

func formatUptime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// HealthController provides health check endpoints.
// For deep checks with DB/Redis, use HealthChecker instead.
type HealthController struct{}

// Check returns a 200 OK if the app is running.
func (c *HealthController) Check(ctx *http.Context) error {
	return ctx.JSON(map[string]string{
		"status": "ok",
	})
}

// Ready returns a 200 OK if the app is ready to receive traffic.
func (c *HealthController) Ready(ctx *http.Context) error {
	return ctx.JSON(map[string]string{
		"status": "ready",
	})
}
