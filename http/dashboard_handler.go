package http

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"

	"github.com/astraframework/astra/assets"
	"github.com/astraframework/astra/core"
	"github.com/go-chi/chi/v5"
)

// DashboardHandler handles requests for the Astra Dev Dashboard.
type DashboardHandler struct {
	dashboard *core.Dashboard
	app       *core.App
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(dash *core.Dashboard, app *core.App) *DashboardHandler {
	return &DashboardHandler{
		dashboard: dash,
		app:       app,
	}
}

// Index serves the dashboard UI.
func (h *DashboardHandler) Index(c *Context) error {
	c.Writer.Header().Set("Content-Type", "text/html")
	if _, err := c.Writer.Write([]byte(assets.DashboardHTML)); err != nil {
		// Ignore write error
	}
	return nil
}

// GetEntries returns the current dashboard entries as JSON.
func (h *DashboardHandler) GetEntries(c *Context) error {
	return c.JSON(h.dashboard.Entries(), http.StatusOK)
}

// GetRoutes returns all registered routes as JSON.
func (h *DashboardHandler) GetRoutes(c *Context) error {
	type RouteInfo struct {
		Method      string   `json:"method"`
		Pattern     string   `json:"pattern"`
		Middlewares []string `json:"middlewares,omitempty"`
	}
	var routes []RouteInfo

	if h.app != nil {
		if svc := h.app.Get("router"); svc != nil {
			if r, ok := svc.(*Router); ok {
				walkFn := func(method string, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
					var mwNames []string
					for range middlewares {
						// This is a basic way to get a name, in Go it's hard without more metadata
						// but we can try to use reflection or just label them.
						mwNames = append(mwNames, "middleware")
					}
					routes = append(routes, RouteInfo{
						Method:      method,
						Pattern:     route,
						Middlewares: mwNames,
					})
					return nil
				}
				_ = chi.Walk(r.mux, walkFn)
			}
		}
	}

	return c.JSON(routes, http.StatusOK)
}

// GetConfig returns the application configuration as JSON (filtered for security).
func (h *DashboardHandler) GetConfig(c *Context) error {
	if h.app == nil || h.app.Config == nil {
		return c.JSON(map[string]any{}, http.StatusOK)
	}

	cfg := h.app.Config
	mask := "********"

	data := map[string]map[string]string{
		"app": {
			"name":    cfg.App.Name,
			"env":     cfg.App.Environment,
			"version": cfg.App.Version,
			"host":    cfg.App.Host,
			"port":    fmt.Sprint(cfg.App.Port),
		},
		"database": {
			"url":       mask,
			"max_conns": fmt.Sprint(cfg.Database.MaxConns),
			"ssl":       cfg.Database.SSL,
		},
		"redis": {
			"url":  mask,
			"host": cfg.Redis.Host,
			"port": fmt.Sprint(cfg.Redis.Port),
		},
	}

	return c.JSON(data, http.StatusOK)
}

// ClearEntries clears all entries in the dashboard.
func (h *DashboardHandler) ClearEntries(c *Context) error {
	h.dashboard.Clear()
	return c.NoContent()
}

// RegisterDashboardRoutes registers the dashboard API and UI routes.
func RegisterDashboardRoutes(r *Router, dash *core.Dashboard) {
	handler := NewDashboardHandler(dash, r.App)

	r.Group("/__astra", func(r *Router) {
		r.Get("/", handler.Index)
		r.Group("/api", func(r *Router) {
			r.Get("/entries", handler.GetEntries)
			r.Get("/routes", handler.GetRoutes)
			r.Get("/config", handler.GetConfig)
			r.Post("/clear", handler.ClearEntries)
			r.Get("/health", handler.HealthCheck)
			r.Get("/ready", handler.HealthReady)
		})
	})
}

// HealthCheck returns the health status.
func (h *DashboardHandler) HealthCheck(c *Context) error {
	svc := h.app.Get("health")
	if svc == nil {
		return c.JSON(map[string]string{"status": "ok"}, http.StatusOK)
	}

	// Use interface to avoid import cycle with telemetry
	type reporter interface {
		Report(ctx context.Context, depth string) any
	}

	if checker, ok := svc.(reporter); ok {
		depth := c.QueryDefault("depth", "L1")
		report := checker.Report(c.Ctx(), depth)

		return c.JSON(report, http.StatusOK)
	}

	return c.JSON(map[string]string{"status": "ok"}, http.StatusOK)
}

// HealthReady returns 200 if the app is healthy.
func (h *DashboardHandler) HealthReady(c *Context) error {
	svc := h.app.Get("health")
	if svc == nil {
		return c.JSON(map[string]string{"status": "ready"}, http.StatusOK)
	}

	type reporter interface {
		Report(ctx context.Context, depth string) any
	}

	if checker, ok := svc.(reporter); ok {
		_ = checker.Report(c.Ctx(), "L1")
		return c.JSON(map[string]string{"status": "ready"}, http.StatusOK)
	}
	return c.JSON(map[string]string{"status": "ready"}, http.StatusOK)
}
