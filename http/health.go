package http

import (
	"context"
	"net/http"

	"github.com/astraframework/astra/telemetry"
)

// HealthHandler returns an HTTP handler that executes all registered health checks
// against internal application components (DB, Redis, etc.) and returns a consolidated JSON report.
// It returns a 200 OK on success, and a 503 Service Unavailable if any component is failing.
func HealthHandler() HandlerFunc {
	return func(c *Context) error {
		if c.App == nil {
			return c.JSON(map[string]string{"status": "unknown"}, http.StatusOK)
		}

		checks := c.App.GetHealthChecks()
		checker := telemetry.NewHealthChecker()
		for name, fnRaw := range checks {
			if fn, ok := fnRaw.(func(context.Context) error); ok {
				checker.Register(name, fn)
			} else if fn, ok := fnRaw.(telemetry.HealthCheckFunc); ok {
				checker.Register(name, fn)
			}
		}

		reportRaw := checker.Report(c.Ctx(), "deep")
		report, ok := reportRaw.(telemetry.HealthReport)
		if !ok {
			return NewHTTPError(http.StatusInternalServerError, "HEALTH_ERROR", "invalid health report format")
		}

		status := http.StatusOK
		if report.Status == telemetry.StatusError {
			status = http.StatusServiceUnavailable
		}

		return c.JSON(report, status)
	}
}

// ReadyHandler returns a simple, fast HTTP handler for liveness probes.
// It simply returns a 200 OK with "OK" text.
func ReadyHandler() HandlerFunc {
	return func(c *Context) error {
		return c.SendString("OK", http.StatusOK)
	}
}
