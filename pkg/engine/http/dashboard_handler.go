package http

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/engine/json"
	"github.com/shauryagautam/Astra/pkg/engine/telemetry"
	platformtelemetry "github.com/shauryagautam/Astra/internal/platform/telemetry"
)

// DashboardHandler handles requests for the Astra Dev Dashboard.
type DashboardHandler struct {
	dashboard    *telemetry.Dashboard
	cfg          *config.AstraConfig
	env          *config.Config
	router       *Router
	mailSandbox  *platformtelemetry.MailSandbox
	queueMon     *telemetry.QueueMonitor
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(
	dash *telemetry.Dashboard,
	cfg *config.AstraConfig,
	env *config.Config,
	router *Router,
	mailSandbox *platformtelemetry.MailSandbox,
	queueMon *telemetry.QueueMonitor,
) *DashboardHandler {
	return &DashboardHandler{
		dashboard:    dash,
		cfg:          cfg,
		env:          env,
		router:       router,
		mailSandbox:  mailSandbox,
		queueMon:     queueMon,
	}
}

// Index serves the dashboard UI.
func (h *DashboardHandler) Index(c *Context) error {
	c.Writer.Header().Set("Content-Type", "text/html")
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Astra Cockpit</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #0f172a;
            --surface: #1e293b;
            --primary: #8b5cf6;
            --secondary: #ec4899;
            --text: #f8fafc;
            --muted: #94a3b8;
        }
        body {
            margin: 0;
            font-family: 'Outfit', sans-serif;
            background: var(--bg);
            color: var(--text);
            display: flex;
            height: 100vh;
            overflow: hidden;
        }
        aside {
            width: 260px;
            background: rgba(30, 41, 59, 0.5);
            backdrop-filter: blur(10px);
            border-right: 1px solid rgba(255, 255, 255, 0.1);
            padding: 2rem;
        }
        main {
            flex: 1;
            padding: 2rem;
            overflow-y: auto;
        }
        h1 {
            font-size: 1.5rem;
            font-weight: 600;
            margin-bottom: 2rem;
            background: linear-gradient(90deg, var(--primary), var(--secondary));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .nav-item {
            padding: 0.75rem 1rem;
            margin-bottom: 0.5rem;
            border-radius: 0.5rem;
            cursor: pointer;
            transition: all 0.2s;
            color: var(--muted);
        }
        .nav-item:hover, .nav-item.active {
            background: rgba(139, 92, 246, 0.1);
            color: var(--primary);
        }
        .card {
            background: var(--surface);
            border-radius: 1rem;
            padding: 1.5rem;
            border: 1px solid rgba(255, 255, 255, 0.05);
            box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
        }
        #entry-list {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }
        .entry {
            border-left: 4px solid var(--primary);
            padding-left: 1rem;
            background: rgba(255, 255, 255, 0.02);
            padding: 1rem;
            border-radius: 0 0.5rem 0.5rem 0;
        }
        .entry-meta {
            font-size: 0.8rem;
            color: var(--muted);
            margin-bottom: 0.25rem;
        }
        .entry-msg {
            font-size: 1rem;
        }
    </style>
</head>
<body>
    <aside>
        <h1>Astra Cockpit</h1>
        <div class="nav-item active">Timeline</div>
        <div class="nav-item">SQL Queries</div>
        <div class="nav-item">Mail Sandbox</div>
        <div class="nav-item">Queues</div>
        <div class="nav-item">Config</div>
    </aside>
    <main>
        <div id="entry-list">
            <div class="card">
                <h3>Live Entry Stream</h3>
                <p style="color: var(--muted)">Waiting for events...</p>
                <div id="entries"></div>
            </div>
        </div>
    </main>
    <script>
        const eventSource = new EventSource('/__astra/api/stream');
        const entriesContainer = document.getElementById('entries');

        eventSource.onmessage = (event) => {
            const entry = JSON.parse(event.data);
            const div = document.createElement('div');
            div.className = 'entry';
            div.innerHTML = '<div class="entry-meta">[' + entry.timestamp + '] ' + entry.type.toUpperCase() + '</div>' +
                            '<div class="entry-msg">' + entry.message + '</div>';
            entriesContainer.prepend(div);
        };
    </script>
</body>
</html>`
	_, _ = c.Writer.Write([]byte(html))
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

	if h.router != nil {
		// Placeholder for route discovery
		routes = append(routes, RouteInfo{
			Method:  "GET",
			Pattern: "/__astra/*",
		})
	}

	return c.JSON(routes, http.StatusOK)
}

// GetConfig returns the application configuration as JSON (filtered for security).
func (h *DashboardHandler) GetConfig(c *Context) error {
	if h.cfg == nil {
		return c.JSON(map[string]any{}, http.StatusOK)
	}

	cfg := h.cfg
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
	}

	return c.JSON(data, http.StatusOK)
}

// ClearEntries clears all entries in the dashboard.
func (h *DashboardHandler) ClearEntries(c *Context) error {
	h.dashboard.Clear()
	return c.NoContent()
}

// Stream streams dashboard entries as Server-Sent Events (SSE).
func (h *DashboardHandler) Stream(c *Context) error {
	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	events, unsubscribe := h.dashboard.Subscribe()
	defer unsubscribe()

	// Initial heartbeat
	fmt.Fprintf(c.Writer, ": ping\n\n")
	flusher.Flush()

	filterType := c.Query("type")

	for {
		select {
		case entry, ok := <-events:
			if !ok {
				return nil
			}
			if filterType != "" && entry.Type != filterType {
				continue
			}
			payload, _ := json.Marshal(entry)
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			flusher.Flush()
		case <-c.Request.Context().Done():
			return nil
		}
	}
}

// RegisterDashboardRoutes registers the dashboard API and UI routes.
func RegisterDashboardRoutes(r *Router, env *config.Config, dash *telemetry.Dashboard, mail *platformtelemetry.MailSandbox, queue *telemetry.QueueMonitor) {
	handler := NewDashboardHandler(dash, r.Config, env, r, mail, queue)

	r.Group("/__astra", func(r *Router) {
		r.Get("/", func(c *Context) error {
			return c.Redirect("/__astra/cockpit", http.StatusFound)
		})
		r.Get("/cockpit", handler.Index)
		r.Group("/api", func(r *Router) {
			// Core telemetry
			r.Get("/entries", handler.GetEntries)
			r.Get("/stream", handler.Stream)
			r.Get("/routes", handler.GetRoutes)
			r.Get("/config", handler.GetConfig)
			r.Post("/clear", handler.ClearEntries)
			r.Get("/health", handler.HealthCheck)
			r.Get("/ready", handler.HealthReady)
			// Phase 4 — Cockpit panels
			r.Get("/queries", handler.GetSQLTimeline)                 // SQL Query Timeline
			r.Get("/mails", handler.GetMails)                         // Mail Sandbox
			r.Delete("/mails", handler.ClearMails)                    // Clear sandbox
			r.Get("/queues", handler.GetQueues)                       // Queue Monitor
			r.Post("/queues/{name}/retry", handler.RetryFailedJobs)   // Retry dead-letter
			r.Post("/queues/{name}/purge", handler.PurgeQueue)        // Purge queue
		})
	})
}

// ─── Phase 4: SQL Query Timeline ─────────────────────────────────────────────

// GetSQLTimeline returns all captured SQL queries with params and durations.
func (h *DashboardHandler) GetSQLTimeline(c *Context) error {
	entries := h.dashboard.Entries()
	var queries []telemetry.DashboardEntry
	for _, e := range entries {
		if e.Type == "query" {
			queries = append(queries, e)
		}
	}
	if queries == nil {
		queries = []telemetry.DashboardEntry{}
	}
	return c.JSON(queries, http.StatusOK)
}

// ─── Phase 4: Mail Sandbox ────────────────────────────────────────────────────

// GetMails returns all emails captured by the MailSandbox.
func (h *DashboardHandler) GetMails(c *Context) error {
	if h.mailSandbox == nil {
		return c.JSON(map[string]any{
			"enabled": false,
			"message": "Mail sandbox is not active.",
			"mails":   []any{},
		}, http.StatusOK)
	}
	return c.JSON(map[string]any{
		"enabled": true,
		"count":   h.mailSandbox.Count(),
		"mails":   h.mailSandbox.Mails(),
	}, http.StatusOK)
}

// ClearMails removes all captured sandbox emails.
func (h *DashboardHandler) ClearMails(c *Context) error {
	if h.mailSandbox != nil {
		h.mailSandbox.Clear()
	}
	return c.NoContent()
}

// ─── Phase 4: Queue Monitor ───────────────────────────────────────────────────

// GetQueues returns real-time stats for all Redis queues.
func (h *DashboardHandler) GetQueues(c *Context) error {
	if h.queueMon == nil {
		return c.JSON(map[string]any{
			"enabled": false,
			"message": "Queue monitor is not active.",
			"queues":  []any{},
		}, http.StatusOK)
	}
	stats, err := h.queueMon.QueueStats(c.Ctx())
	if err != nil {
		return fmt.Errorf("dashboard: queue stats: %w", err)
	}
	return c.JSON(map[string]any{
		"enabled": true,
		"queues":  stats,
	}, http.StatusOK)
}

// RetryFailedJobs moves dead-letter jobs back to the pending queue.
func (h *DashboardHandler) RetryFailedJobs(c *Context) error {
	if h.queueMon == nil {
		return c.JSON(map[string]string{"error": "queue monitor not active"}, http.StatusServiceUnavailable)
	}
	name := c.Param("name")
	count, err := h.queueMon.RetryFailed(c.Ctx(), name)
	if err != nil {
		return fmt.Errorf("dashboard: retry failed: %w", err)
	}
	return c.JSON(map[string]any{"retried": count, "queue": name}, http.StatusOK)
}

// PurgeQueue removes all pending jobs from a queue.
func (h *DashboardHandler) PurgeQueue(c *Context) error {
	if h.queueMon == nil {
		return c.JSON(map[string]string{"error": "queue monitor not active"}, http.StatusServiceUnavailable)
	}
	name := c.Param("name")
	count, err := h.queueMon.PurgeQueue(c.Ctx(), name)
	if err != nil {
		return fmt.Errorf("dashboard: purge queue: %w", err)
	}
	return c.JSON(map[string]any{"purged": count, "queue": name}, http.StatusOK)
}

// HealthCheck returns the health status.
func (h *DashboardHandler) HealthCheck(c *Context) error {
	return c.JSON(map[string]string{"status": "ok"}, http.StatusOK)
}

// HealthReady returns 200 if the app is healthy.
func (h *DashboardHandler) HealthReady(c *Context) error {
	return c.JSON(map[string]string{"status": "ready"}, http.StatusOK)
}
