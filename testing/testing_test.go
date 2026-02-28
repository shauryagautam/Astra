package adonistesting

import (
	"testing"

	adonisHttp "github.com/shaurya/adonis/app/Http"
	"github.com/shaurya/adonis/contracts"
)

func TestTestAppBootstrap(t *testing.T) {
	app := NewTestApp()
	if app.App == nil {
		t.Fatal("App should not be nil")
	}
	if app.Router == nil {
		t.Fatal("Router should not be nil")
	}
	if app.Server == nil {
		t.Fatal("Server should not be nil")
	}
}

func TestGetRequest(t *testing.T) {
	app := NewTestApp()
	app.RegisterRoutes(func(router *adonisHttp.Router) {
		router.Get("/test", func(ctx contracts.HttpContextContract) error {
			return ctx.Response().Json(map[string]any{
				"message": "hello test",
			})
		})
	})

	resp := app.Get("/test").Expect(t)
	resp.AssertOk()
	resp.AssertContains("hello test")
	resp.AssertJson("message", "hello test")
}

func TestPostRequestWithJSON(t *testing.T) {
	app := NewTestApp()
	app.RegisterRoutes(func(router *adonisHttp.Router) {
		router.Post("/echo", func(ctx contracts.HttpContextContract) error {
			return ctx.Response().Status(201).Json(map[string]any{
				"received": true,
			})
		})
	})

	resp := app.Post("/echo").
		WithJSON(map[string]string{"name": "John"}).
		Expect(t)

	resp.AssertCreated()
	resp.AssertJson("received", true)
}

func TestNotFoundRoute(t *testing.T) {
	app := NewTestApp()
	resp := app.Get("/nonexistent").Expect(t)
	resp.AssertNotFound()
}

func TestAssertJsonHasKey(t *testing.T) {
	app := NewTestApp()
	app.RegisterRoutes(func(router *adonisHttp.Router) {
		router.Get("/keys", func(ctx contracts.HttpContextContract) error {
			return ctx.Response().Json(map[string]any{
				"name":  "John",
				"email": "john@example.com",
			})
		})
	})

	resp := app.Get("/keys").Expect(t)
	resp.AssertOk()
	resp.AssertJsonHasKey("name")
	resp.AssertJsonHasKey("email")
}

func TestWithHeader(t *testing.T) {
	app := NewTestApp()
	app.RegisterRoutes(func(router *adonisHttp.Router) {
		router.Get("/header-check", func(ctx contracts.HttpContextContract) error {
			val := ctx.Request().Header("X-Custom")
			return ctx.Response().Json(map[string]any{"custom": val})
		})
	})

	resp := app.Get("/header-check").
		WithHeader("X-Custom", "test-value").
		Expect(t)

	resp.AssertOk()
	resp.AssertJson("custom", "test-value")
}

func TestNewTestContext(t *testing.T) {
	ctx, w := NewTestContext("GET", "/test")
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	if w == nil {
		t.Fatal("recorder should not be nil")
	}

	// Use the context to write a response
	err := ctx.Response().Json(map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("Response.Json failed: %v", err)
	}

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}
