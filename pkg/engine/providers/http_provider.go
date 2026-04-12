package providers

import (
	"context"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
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
func (p *HTTPProvider) Register(app *engine.App) error {
	// Add default security headers
	if router, ok := p.Handler.(*astrahttp.Router); ok {
		isProd := app.Env().IsProd()
		router.Use(astrahttp.SecureHeaders(isProd))
	}

	return nil
}

// Boot is a no-op for HTTPProvider.
func (p *HTTPProvider) Boot(app *engine.App) error {
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (p *HTTPProvider) Shutdown(ctx context.Context, app *engine.App) error {
	return nil
}
