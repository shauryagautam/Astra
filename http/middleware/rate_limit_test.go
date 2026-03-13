package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	astrahttp "github.com/astraframework/astra/http"
	goredis "github.com/redis/go-redis/v9"
)

func setupRateLimitClient(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	return server, client
}

func TestRateLimitAllowsRequests(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 2, time.Minute)
	if err != nil {
		t.Fatalf("failed to create rate limit middleware: %v", err)
	}
	handler := middleware(func(c *astrahttp.Context) error {
		return c.JSON(map[string]string{"status": "ok"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.10:1234"

	ctx := astrahttp.NewContext(recorder, request, nil)
	if err := handler(ctx); err != nil {
		t.Fatalf("handle request: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-RateLimit-Limit") != "2" {
		t.Fatalf("unexpected limit header: %q", recorder.Header().Get("X-RateLimit-Limit"))
	}
	if recorder.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Fatalf("unexpected remaining header: %q", recorder.Header().Get("X-RateLimit-Remaining"))
	}
	if recorder.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatal("missing reset header")
	}
}

func TestRateLimitExceeded(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 1, time.Minute)
	if err != nil {
		t.Fatalf("failed to create rate limit middleware: %v", err)
	}
	handler := middleware(func(c *astrahttp.Context) error {
		return c.JSON(map[string]string{"status": "ok"})
	})

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "198.51.100.7:1234"

		ctx := astrahttp.NewContext(recorder, request, nil)
		err := handler(ctx)
		if attempt == 0 {
			if err != nil {
				t.Fatalf("unexpected error on first request: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error on second request: %v", err)
		}
		if recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", recorder.Code)
		}
		if recorder.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After header")
		}

		var body map[string]any
		if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &body); decodeErr != nil {
			t.Fatalf("decode response: %v", decodeErr)
		}
		if body["error"] != "rate_limit_exceeded" {
			t.Fatalf("unexpected error body: %#v", body)
		}
	}
}

func TestRateLimitCanceledContext(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 1, time.Minute)
	if err != nil {
		t.Fatalf("failed to create rate limit middleware: %v", err)
	}
	handler := middleware(func(c *astrahttp.Context) error {
		return c.JSON(map[string]string{"status": "ok"})
	})

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(reqCtx)
	request.RemoteAddr = "192.0.2.25:1234"

	ctx := astrahttp.NewContext(recorder, request, nil)
	handleErr := handler(ctx)
	if !errors.Is(handleErr, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", handleErr)
	}
}

func TestTokenBucketRateLimitExceeded(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 1, time.Minute, WithAlgorithm(astrahttp.TokenBucket))
	if err != nil {
		t.Fatalf("failed to create rate limit middleware: %v", err)
	}
	handler := middleware(func(c *astrahttp.Context) error {
		return c.JSON(map[string]string{"status": "ok"})
	})

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "198.51.100.8:1234"

		ctx := astrahttp.NewContext(recorder, request, nil)
		err := handler(ctx)
		if attempt == 0 {
			if err != nil {
				t.Fatalf("unexpected error on first request: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error on second request: %v", err)
		}
		if recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", recorder.Code)
		}
		if recorder.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After header")
		}

		var body map[string]any
		if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &body); decodeErr != nil {
			t.Fatalf("decode response: %v", decodeErr)
		}
		if body["error"] != "rate_limit_exceeded" {
			t.Fatalf("unexpected error body: %#v", body)
		}
	}
}
