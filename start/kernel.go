// Package start — kernel.go is the equivalent of Astra's start/kernel.ts.
// It configures global and named middleware.
package start

import (
	astraHttp "github.com/shaurya/astra/app/Http"
	"github.com/shaurya/astra/config"
	"github.com/shaurya/astra/contracts"
)

// RegisterMiddleware registers global and named middleware on the server.
// Mirrors Astra's start/kernel.ts.
//
// In Astra:
//
//	Server.middleware.register([
//	  () => import('@ioc:Astra/Core/BodyParser'),
//	  () => import('App/Middleware/SilentAuth'),
//	])
//
//	Server.middleware.registerNamed({
//	  auth: () => import('App/Middleware/Auth'),
//	})
func RegisterMiddleware(app contracts.ApplicationContract, corsConfig config.CorsConfig) {
	server := app.Use("Server").(*astraHttp.Server)

	// ── Global Middleware (applied to all routes) ──────────────────────
	if corsConfig.Enabled {
		server.Use(astraHttp.CorsMiddleware(
			corsConfig.Origin,
			corsConfig.Methods,
			corsConfig.Headers,
		))
	}

	// ── Named Middleware (referenced by name in routes) ────────────────
	// Example: Route.get('/dashboard', handler).middleware('auth')
	// Named middleware will be registered here as auth, JWT, etc. modules are built.
}
