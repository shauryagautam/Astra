// Package start — kernel.go is the equivalent of AdonisJS's start/kernel.ts.
// It configures global and named middleware.
package start

import (
	adonisHttp "github.com/shaurya/adonis/app/Http"
	"github.com/shaurya/adonis/config"
	"github.com/shaurya/adonis/contracts"
)

// RegisterMiddleware registers global and named middleware on the server.
// Mirrors AdonisJS's start/kernel.ts.
//
// In AdonisJS:
//
//	Server.middleware.register([
//	  () => import('@ioc:Adonis/Core/BodyParser'),
//	  () => import('App/Middleware/SilentAuth'),
//	])
//
//	Server.middleware.registerNamed({
//	  auth: () => import('App/Middleware/Auth'),
//	})
func RegisterMiddleware(app contracts.ApplicationContract, corsConfig config.CorsConfig) {
	server := app.Use("Server").(*adonisHttp.Server)

	// ── Global Middleware (applied to all routes) ──────────────────────
	if corsConfig.Enabled {
		server.Use(adonisHttp.CorsMiddleware(
			corsConfig.Origin,
			corsConfig.Methods,
			corsConfig.Headers,
		))
	}

	// ── Named Middleware (referenced by name in routes) ────────────────
	// Example: Route.get('/dashboard', handler).middleware('auth')
	// Named middleware will be registered here as auth, JWT, etc. modules are built.
}
