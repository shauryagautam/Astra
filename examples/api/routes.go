package main

import (
	"github.com/shaurya/astra/contracts"
)

func registerRoutes(router contracts.RouterContract) {
	// Controllers
	usersCtrl := &UsersController{}
	healthCtrl := &HealthController{}

	// Root route
	router.Get("/", func(ctx contracts.HttpContextContract) error {
		return ctx.Response().Json(map[string]any{
			"framework": "Astra",
			"message":   "Welcome to the production-ready Astra API! 🚀",
		})
	})

	// Health check
	router.Get("/health", healthCtrl.Check)

	// API v1 Group
	router.Group(func(api contracts.RouterContract) {
		api.Get("/users", usersCtrl.Index)
		api.Post("/users", usersCtrl.Store)
		api.Get("/users/:id", usersCtrl.Show)

		// Error demo route
		api.Get("/error", func(ctx contracts.HttpContextContract) error {
			panic("Something went wrong! (This panic is caught by the recovery middleware)")
		})
	}).Prefix("/api/v1")
}
