// Package start is the equivalent of AdonisJS's start/ directory.
// This package contains route definitions and kernel configuration.
package start

import (
	adonisHttp "github.com/shaurya/adonis/app/Http"
	"github.com/shaurya/adonis/contracts"
)

// RegisterRoutes registers all application routes.
// This is the equivalent of AdonisJS's start/routes.ts file.
//
// In AdonisJS:
//
//	Route.get('/', async ({ response }) => {
//	  response.json({ hello: 'world' })
//	})
//
// In Adonis Go:
//
//	Route.Get("/", func(ctx contracts.HttpContextContract) error {
//	  return ctx.Response().Json(map[string]any{"hello": "world"})
//	})
func RegisterRoutes(app contracts.ApplicationContract) {
	Route := app.Use("Route").(*adonisHttp.Router)

	// ── Default Routes ─────────────────────────────────────────────────
	Route.Get("/", func(ctx contracts.HttpContextContract) error {
		return ctx.Response().Json(map[string]any{
			"framework": "Adonis Go",
			"version":   app.Version(),
			"message":   "Server is running! 🚀",
		})
	}).As("home")

	// ── Health Check ───────────────────────────────────────────────────
	Route.Get("/health", func(ctx contracts.HttpContextContract) error {
		return ctx.Response().Json(map[string]any{
			"status": "healthy",
			"uptime": "ok",
		})
	}).As("health")

	// ── API v1 Group ───────────────────────────────────────────────────
	Route.Group(func(group contracts.RouterContract) {
		group.Get("/status", func(ctx contracts.HttpContextContract) error {
			return ctx.Response().Json(map[string]any{
				"api":    "v1",
				"status": "active",
			})
		}).As("api.v1.status")
	}).Prefix("/api/v1")
}
