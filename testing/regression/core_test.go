package regression

import (
	"os"
	"testing"

	"github.com/astraframework/astra/core"
	astrahttp "github.com/astraframework/astra/http"
	"github.com/astraframework/astra/http/middleware"
	astratest "github.com/astraframework/astra/testing"
	"github.com/stretchr/testify/assert"
)

// TestCSRFProtection_Guarantees ensures that state-changing requests without CSRF tokens are rejected.
func TestCSRFProtection_Guarantees(t *testing.T) {
	app := astratest.NewTestApp(t, func(app *core.App, r *astrahttp.Router) {
		r.Use(middleware.CSRF())
		r.Post("/test", func(c *astrahttp.Context) error {
			return c.NoContent()
		})
	})

	// Unauthorized request (missing token)
	app.POST("/test", "").AssertStatus(403)
}

// TestMiddlewareOrdering ensures that middleware are executed in the registered order.
func TestMiddlewareOrdering(t *testing.T) {
	var executionOrder []string

	app := astratest.NewTestApp(t, func(app *core.App, r *astrahttp.Router) {
		r.Use(func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
			return func(c *astrahttp.Context) error {
				executionOrder = append(executionOrder, "first")
				return next(c)
			}
		})
		r.Use(func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
			return func(c *astrahttp.Context) error {
				executionOrder = append(executionOrder, "second")
				return next(c)
			}
		})
		r.Get("/test", func(c *astrahttp.Context) error {
			return c.NoContent()
		})
	})

	app.GET("/test").AssertStatus(200)
	assert.Equal(t, []string{"first", "second"}, executionOrder)
}

// TestSecureHeaders_InProduction ensures that secure headers are enforced in production environment.
func TestSecureHeaders_InProduction(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	defer os.Setenv("APP_ENV", "test")

	app := astratest.NewTestApp(t, func(app *core.App, r *astrahttp.Router) {
		// SecureHeaders is already in the default stack for NewRouter,
		// but we can re-apply or just test the defaults if app is in prod.
		r.Get("/test", func(c *astrahttp.Context) error {
			return c.NoContent()
		})
	})

	resp := app.GET("/test")
	resp.AssertHeader("X-Frame-Options", "DENY")
	resp.AssertHeader("X-Content-Type-Options", "nosniff")
	resp.AssertHeader("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
