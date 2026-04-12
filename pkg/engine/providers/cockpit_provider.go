package providers

import (
	"log/slog"
	"time"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/http"
	"github.com/shauryagautam/Astra/pkg/engine/telemetry"
	platformtelemetry "github.com/shauryagautam/Astra/internal/platform/telemetry"
	"github.com/shauryagautam/Astra/pkg/database"
)

type CockpitProvider struct {
	engine.BaseProvider
	dash     *telemetry.Dashboard
	sandbox  *platformtelemetry.MailSandbox
	queueMon *telemetry.QueueMonitor
	db       *database.DB
	router   *http.Router
}

func NewCockpitProvider(
	dash *telemetry.Dashboard,
	sandbox *platformtelemetry.MailSandbox,
	queueMon *telemetry.QueueMonitor,
	db *database.DB,
	router *http.Router,
) *CockpitProvider {
	return &CockpitProvider{
		dash:     dash,
		sandbox:  sandbox,
		queueMon: queueMon,
		db:       db,
		router:   router,
	}
}

func (p *CockpitProvider) Name() string { return "cockpit" }

func (p *CockpitProvider) Register(app *engine.App) error {
	// Only activate Cockpit in non-production environments
	if app.Env().IsProd() {
		return nil
	}

	slog.Info("✓ Cockpit developer services registered")
	return nil
}

func (p *CockpitProvider) Boot(app *engine.App) error {
	if app.Env().IsProd() {
		return nil
	}

	// 1. Wire Database Query Timeline
	if p.db != nil {
		p.db.SetQueryHook(func(sql string, args []any, d time.Duration) {
			p.dash.TrackQuery(sql, args, d)
		})
		slog.Info("cockpit: database query timeline active")
	}

	// 2. Register Dashboard Routes
	if p.router != nil {
		http.RegisterDashboardRoutes(p.router, app.Env(), p.dash, p.sandbox, p.queueMon)
		slog.Info("cockpit: developer dashboard routes registered at /__astra")
	}

	return nil
}

