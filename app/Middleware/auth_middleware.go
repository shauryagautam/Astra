package middleware

import (
	"github.com/shaurya/adonis/contracts"
)

// AuthMiddleware returns middleware that requires authentication.
// Mirrors AdonisJS's auth middleware: Route.get('/dashboard', handler).middleware('auth')
//
// Usage in kernel.go:
//
//	server.RegisterNamed("auth", middleware.AuthMiddleware(authManager, "api"))
func AuthMiddleware(authManager contracts.AuthManagerContract, guardName ...string) contracts.MiddlewareFunc {
	return func(ctx contracts.HttpContextContract, next func() error) error {
		var guard contracts.GuardContract
		if len(guardName) > 0 {
			guard = authManager.Use(guardName[0])
		} else {
			guard = authManager.DefaultGuard()
		}

		user, err := guard.Authenticate(ctx)
		if err != nil {
			return ctx.Response().Status(401).Json(map[string]any{
				"error":   "Unauthorized",
				"message": "Authentication required",
				"status":  401,
			})
		}

		// Store authenticated user in context
		ctx.WithValue("auth_user", user)
		ctx.WithValue("auth_guard", guard)

		return next()
	}
}

// GuestMiddleware returns middleware that only allows unauthenticated requests.
// Mirrors AdonisJS's guest middleware.
func GuestMiddleware(authManager contracts.AuthManagerContract, guardName ...string) contracts.MiddlewareFunc {
	return func(ctx contracts.HttpContextContract, next func() error) error {
		var guard contracts.GuardContract
		if len(guardName) > 0 {
			guard = authManager.Use(guardName[0])
		} else {
			guard = authManager.DefaultGuard()
		}

		if guard.Check(ctx) {
			return ctx.Response().Status(400).Json(map[string]any{
				"error":   "Bad Request",
				"message": "You are already authenticated",
				"status":  400,
			})
		}

		return next()
	}
}

// SilentAuthMiddleware silently tries to authenticate without blocking.
// If token is present and valid, sets the user. If not, continues anyway.
func SilentAuthMiddleware(authManager contracts.AuthManagerContract, guardName ...string) contracts.MiddlewareFunc {
	return func(ctx contracts.HttpContextContract, next func() error) error {
		var guard contracts.GuardContract
		if len(guardName) > 0 {
			guard = authManager.Use(guardName[0])
		} else {
			guard = authManager.DefaultGuard()
		}

		// Try to authenticate, ignore errors
		if user, err := guard.Authenticate(ctx); err == nil {
			ctx.WithValue("auth_user", user)
		}

		return next()
	}
}
