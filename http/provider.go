package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/astraframework/astra/core"
)

// HTTPProvider manages the lifecycle of the HTTP server.
type HTTPProvider struct {
	Handler http.Handler
}

// NewHTTPProvider creates a new HTTPProvider with the given handler.
func NewHTTPProvider(handler http.Handler) *HTTPProvider {
	return &HTTPProvider{Handler: handler}
}

// Name returns the provider name.
func (p *HTTPProvider) Name() string {
	return "http"
}

// Register registers the HTTP server as a service.
func (p *HTTPProvider) Register(app *core.App) error {
	addr := fmt.Sprintf("%s:%d", app.Config.App.Host, app.Config.App.Port)
	server := NewServer(addr, p.Handler)
	app.Register("http_server", server)

	// Add default security headers
	if router, ok := p.Handler.(*Router); ok {
		router.Use(SecureHeaders())
	}

	return nil
}

// Boot is a no-op for HTTPProvider.
func (p *HTTPProvider) Boot(app *core.App) error {
	return nil
}

// Ready is a no-op for HTTPProvider.
func (p *HTTPProvider) Ready(app *core.App) error {
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (p *HTTPProvider) Shutdown(ctx context.Context, app *core.App) error {
	svc := app.Get("http_server")
	if svc == nil {
		return nil
	}

	server, ok := svc.(*Server)
	if !ok {
		return nil
	}

	app.Logger.Info("stopping HTTP server...")
	return server.Shutdown(ctx)
}
