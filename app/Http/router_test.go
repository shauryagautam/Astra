package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shaurya/adonis/contracts"
)

func TestRouterGet(t *testing.T) {
	router := NewRouter()
	router.Get("/hello", func(ctx contracts.HttpContextContract) error {
		return ctx.Response().Json(map[string]string{"msg": "hello"})
	})

	route, params, found := router.FindRoute("GET", "/hello")
	if !found {
		t.Fatal("GET /hello should be found")
	}
	if route.Pattern() != "/hello" {
		t.Fatalf("expected pattern '/hello', got '%s'", route.Pattern())
	}
	if len(params) != 0 {
		t.Fatal("expected no params")
	}
}

func TestRouterParams(t *testing.T) {
	router := NewRouter()
	router.Get("/users/:id", func(ctx contracts.HttpContextContract) error {
		return nil
	})

	route, params, found := router.FindRoute("GET", "/users/42")
	if !found {
		t.Fatal("GET /users/42 should match /users/:id")
	}
	if route.Pattern() != "/users/:id" {
		t.Fatalf("expected pattern '/users/:id', got '%s'", route.Pattern())
	}
	if params["id"] != "42" {
		t.Fatalf("expected param id=42, got '%s'", params["id"])
	}
}

func TestRouterNotFound(t *testing.T) {
	router := NewRouter()
	router.Get("/hello", func(ctx contracts.HttpContextContract) error {
		return nil
	})

	_, _, found := router.FindRoute("GET", "/nonexistent")
	if found {
		t.Fatal("should not find /nonexistent")
	}
}

func TestRouterMethodMismatch(t *testing.T) {
	router := NewRouter()
	router.Get("/hello", func(ctx contracts.HttpContextContract) error {
		return nil
	})

	_, _, found := router.FindRoute("POST", "/hello")
	if found {
		t.Fatal("POST /hello should not match GET route")
	}
}

func TestRouterGroup(t *testing.T) {
	router := NewRouter()
	router.Group(func(g contracts.RouterContract) {
		g.Get("/status", func(ctx contracts.HttpContextContract) error {
			return nil
		})
	}).Prefix("/api/v1")

	_, _, found := router.FindRoute("GET", "/api/v1/status")
	if !found {
		t.Fatal("GET /api/v1/status should match grouped route")
	}
}

func TestRouterGroupMiddleware(t *testing.T) {
	router := NewRouter()
	router.Group(func(g contracts.RouterContract) {
		g.Get("/dashboard", func(ctx contracts.HttpContextContract) error {
			return nil
		})
	}).Prefix("/admin").Middleware("auth")

	route, _, found := router.FindRoute("GET", "/admin/dashboard")
	if !found {
		t.Fatal("GET /admin/dashboard should match")
	}
	mw := route.GetMiddleware()
	if len(mw) != 1 || mw[0] != "auth" {
		t.Fatalf("expected middleware ['auth'], got %v", mw)
	}
}

func TestRouterResource(t *testing.T) {
	ctrl := &testResourceController{}
	router := NewRouter()
	router.Resource("posts", ctrl)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/posts"},
		{"POST", "/posts"},
		{"GET", "/posts/1"},
		{"PUT", "/posts/1"},
		{"DELETE", "/posts/1"},
	}

	for _, tt := range tests {
		_, _, found := router.FindRoute(tt.method, tt.path)
		if !found {
			t.Fatalf("%s %s should match resource route", tt.method, tt.path)
		}
	}
}

func TestRouteNaming(t *testing.T) {
	router := NewRouter()
	router.Get("/users", func(ctx contracts.HttpContextContract) error {
		return nil
	}).As("users.list")

	route, _, found := router.FindRoute("GET", "/users")
	if !found {
		t.Fatal("should find route")
	}
	if route.Name() != "users.list" {
		t.Fatalf("expected name 'users.list', got '%s'", route.Name())
	}
}

func TestRouteMiddleware(t *testing.T) {
	router := NewRouter()
	router.Get("/secret", func(ctx contracts.HttpContextContract) error {
		return nil
	}).Middleware("auth", "throttle")

	route, _, _ := router.FindRoute("GET", "/secret")
	mw := route.GetMiddleware()
	if len(mw) != 2 {
		t.Fatalf("expected 2 middleware, got %d", len(mw))
	}
}

func TestHttpContextIntegration(t *testing.T) {
	// Test the full chain: router → server → context
	router := NewRouter()
	router.Get("/echo/:name", func(ctx contracts.HttpContextContract) error {
		name := ctx.Param("name")
		return ctx.Response().Json(map[string]string{"name": name})
	})

	server := NewServer()
	server.SetRouter(router)

	req := httptest.NewRequest("GET", "/echo/john", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body")
	}
}

func TestRouterCommit(t *testing.T) {
	router := NewRouter()
	router.Get("/static", func(ctx contracts.HttpContextContract) error { return nil })
	router.Get("/users/:id", func(ctx contracts.HttpContextContract) error { return nil })

	// Before commit, should work via fallback
	_, _, found := router.FindRoute("GET", "/static")
	if !found {
		t.Fatal("Should find static route before commit")
	}

	router.Commit()

	// After commit, should work via optimized maps
	_, _, found = router.FindRoute("GET", "/static")
	if !found {
		t.Fatal("Should find static route after commit")
	}

	_, params, found := router.FindRoute("GET", "/users/123")
	if !found || params["id"] != "123" {
		t.Fatal("Should find dynamic route after commit")
	}
}

func BenchmarkFindRouteStatic(b *testing.B) {
	router := NewRouter()
	router.Get("/api/v1/users/profile/settings", func(ctx contracts.HttpContextContract) error { return nil })
	router.Commit()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.FindRoute("GET", "/api/v1/users/profile/settings")
	}
}

func BenchmarkFindRouteDynamic(b *testing.B) {
	router := NewRouter()
	router.Get("/api/v1/users/:id/posts/:post_id", func(ctx contracts.HttpContextContract) error { return nil })
	router.Commit()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.FindRoute("GET", "/api/v1/users/123/posts/456")
	}
}

// testResourceController implements ResourceController for testing.
type testResourceController struct{}

func (c *testResourceController) Index(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]string{"action": "index"})
}
func (c *testResourceController) Store(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]string{"action": "store"})
}
func (c *testResourceController) Show(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]string{"action": "show"})
}
func (c *testResourceController) Update(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]string{"action": "update"})
}
func (c *testResourceController) Destroy(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]string{"action": "destroy"})
}
