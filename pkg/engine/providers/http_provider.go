package providers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
)

// HTTPProvider manages the lifecycle of the HTTP server.
type HTTPProvider struct {
	engine.BaseProvider
	Handler http.Handler
	server  *astrahttp.Server
}

// NewHTTPProvider creates a new HTTPProvider with the given handler.
func NewHTTPProvider(handler http.Handler) *HTTPProvider {
	return &HTTPProvider{Handler: handler}
}

// Name returns the provider name.
func (p *HTTPProvider) Name() string {
	return "http"
}

// GetRouter returns the injected HTTP handler.
func (p *HTTPProvider) GetRouter() any {
	return p.Handler
}

// Register registers the HTTP server as a service.
func (p *HTTPProvider) Register(app *engine.App) error {
	// Add default security headers
	if router, ok := p.Handler.(*astrahttp.Router); ok {
		isProd := app.Env().IsProd()
		router.Use(astrahttp.SecureHeaders(isProd))
	}

	return nil
}

// Boot starts the HTTP server dynamically.
func (p *HTTPProvider) Boot(app *engine.App) error {
	addr := fmt.Sprintf("%s:%d", app.Config().App.Host, app.Config().App.Port)
	if app.Config().App.Host == "" && app.Config().App.Port == 0 {
		addr = ":8080"
	}
	p.server = astrahttp.NewServer(addr, p.Handler)
	app.Logger().Info("Starting HTTP server", "addr", addr)
	return p.server.Start(app.BaseContext())
}

// Shutdown gracefully stops the HTTP server.
func (p *HTTPProvider) Shutdown(ctx context.Context, app *engine.App) error {
	if p.server != nil {
		app.Logger().Info("Shutting down HTTP server")
		return p.server.Shutdown(ctx)
	}
	return nil
}
