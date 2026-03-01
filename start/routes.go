// Package start is the equivalent of Astra's start/ directory.
// This package contains route definitions and kernel configuration.
package start

import (
	"database/sql"

	astraHttp "github.com/shaurya/astra/app/Http"
	"github.com/shaurya/astra/contracts"
)

// RegisterRoutes registers all application routes.
// This is the equivalent of Astra's start/routes.ts file.
//
// In Astra:
//
//	Route.get('/', async ({ response }) => {
//	  response.json({ hello: 'world' })
//	})
//
// In Astra Go:
//
//	Route.Get("/", func(ctx contracts.HttpContextContract) error {
//	  return ctx.Response().Json(map[string]any{"hello": "world"})
//	})
func RegisterRoutes(app contracts.ApplicationContract) {
	Route := app.Use("Route").(*astraHttp.Router)

	// ── Default Routes ─────────────────────────────────────────────────
	Route.Get("/", func(ctx contracts.HttpContextContract) error {
		return ctx.Response().Json(map[string]any{
			"framework": "Astra Go",
			"version":   app.Version(),
			"message":   "Server is running! 🚀",
		})
	}).As("home")

	// ── Health Check ───────────────────────────────────────────────────
	Route.Get("/health", func(ctx contracts.HttpContextContract) error {
		status := "healthy"
		details := map[string]any{
			"uptime": "ok",
		}

		// Try pinging DB and Redis if they are bound
		if app.HasBinding("Database") {
			if db, err := app.Make("Database"); err == nil {
				if gormDB, ok := db.(interface{ DB() (*sql.DB, error) }); ok {
					if _, err := gormDB.DB(); err != nil {
						status = "degraded"
						details["database"] = "unreachable"
					} else {
						details["database"] = "ok"
					}
				}
			}
		}

		if app.HasBinding("Redis") {
			if redisMgr, err := app.Make("Redis"); err == nil {
				if mgr, ok := redisMgr.(contracts.RedisContract); ok {
					if err := mgr.Connection("local").Ping(ctx.Context()); err != nil {
						status = "degraded"
						details["redis"] = "unreachable"
					} else {
						details["redis"] = "ok"
					}
				}
			}
		}

		details["status"] = status
		return ctx.Response().Json(details)
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
