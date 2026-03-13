package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouterGroupInheritsHooksAndNamedRoutes(t *testing.T) {
	router := NewRouter(nil)

	var beforeCount int
	var afterCount int

	router.Before(func(c *Context) error {
		beforeCount++
		return nil
	})
	router.After(func(c *Context) error {
		afterCount++
		return nil
	})

	router.Group("/api", func(r *Router) {
		r.Get("/users", func(c *Context) error {
			return c.SendString("ok", http.StatusOK)
		}).Name("users.index")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
	require.Equal(t, 1, beforeCount)
	require.Equal(t, 1, afterCount)
	require.Equal(t, "/api/users", router.Route("users.index"))
}

func TestRouter_Middleware(t *testing.T) {
	router := NewRouter(nil)
	var trace []string

	router.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			trace = append(trace, "m1-start")
			err := next(c)
			trace = append(trace, "m1-end")
			return err
		}
	})

	router.Get("/ping", func(c *Context) error {
		trace = append(trace, "handler")
		return c.SendString("pong", http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, []string{"m1-start", "handler", "m1-end"}, trace)
}

func TestRouter_ErrorHandling(t *testing.T) {
	router := NewRouter(nil)

	router.Get("/error", func(c *Context) error {
		return fmt.Errorf("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "INTERNAL_ERROR")
}
