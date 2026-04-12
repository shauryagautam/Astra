package regression

import (
	"net/http"
	"testing"

	"github.com/shauryagautam/Astra/pkg/engine"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
	astratest "github.com/shauryagautam/Astra/pkg/test_util"
	"github.com/stretchr/testify/assert"
)

// TestCSRFProtection_Guarantees ensures that state-changing requests without CSRF tokens are rejected.
func TestCSRFProtection_Guarantees(t *testing.T) {
	app := astratest.NewTestApp(t, func(app *engine.App, r *astrahttp.Router) {
		r.Use(astrahttp.CSRF(false))
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

	app := astratest.NewTestApp(t, func(app *engine.App, r *astrahttp.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionOrder = append(executionOrder, "first")
				next.ServeHTTP(w, r)
			})
		})
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionOrder = append(executionOrder, "second")
				next.ServeHTTP(w, r)
			})
		})
		r.Get("/test", func(c *astrahttp.Context) error {
			return c.NoContent()
		})
	})

	app.GET("/test").AssertStatus(204)
	assert.Equal(t, []string{"first", "second"}, executionOrder)
}

// TestSecureHeaders ensures that secure headers are enforced correctly.
func TestSecureHeaders(t *testing.T) {
	app := astratest.NewTestApp(t, func(app *engine.App, r *astrahttp.Router) {
		r.Use(astrahttp.SecureHeaders(false))
		r.Get("/test", func(c *astrahttp.Context) error {
			return c.NoContent()
		})
	})

	resp := app.GET("/test")
	// SSR defaults use SAMEORIGIN
	resp.AssertHeader("X-Frame-Options", "SAMEORIGIN")
	resp.AssertHeader("X-Content-Type-Options", "nosniff")
	// HSTS is not sent in test environment by default
}
