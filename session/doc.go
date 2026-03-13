// Package session provides a flexible, secure HTTP session layer for the Astra framework.
//
// It supports multiple backend drivers via the `Store` interface, shipping with both
// a zero-state, AES-256-GCM encrypted `CookieStore` and a fast, stateful `RedisStore`.
//
// Sessions in Astra are lazily decoded and automatically persisted at the end of the
// request lifecycle via the session Middleware.
//
// Example usage:
//
//	// Set values and flash messages
//	sess := ctx.Session()
//	sess.Set("user_id", 123)
//	sess.SetFlash("success", "Logged in successfully!")
//
//	// Redirecting will auto-save the session state
//	return ctx.Redirect(302, "/dashboard")
package session
