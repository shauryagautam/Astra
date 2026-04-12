package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler := middleware(next)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.10:1234"

	c := NewContext(recorder, request)
	ctx := context.WithValue(request.Context(), astraContextKey, c)
	request = request.WithContext(ctx)

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "2", recorder.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "1", recorder.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, recorder.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimitExceeded(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 1, time.Minute)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler := middleware(next)

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "198.51.100.7:1234"

		c := NewContext(recorder, request)
		ctx := context.WithValue(request.Context(), astraContextKey, c)
		request = request.WithContext(ctx)

		handler.ServeHTTP(recorder, request)

		if attempt == 0 {
			assert.Equal(t, http.StatusOK, recorder.Code)
			continue
		}

		assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
		assert.NotEmpty(t, recorder.Header().Get("Retry-After"))

		var body map[string]any
		err := json.Unmarshal(recorder.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, "rate_limit_exceeded", body["error"])
	}
}

func TestRateLimitCanceledContext(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 1, time.Minute)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	handler := middleware(next)

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(reqCtx)
	request.RemoteAddr = "192.0.2.25:1234"

	c := NewContext(recorder, request)
	ctx := context.WithValue(request.Context(), astraContextKey, c)
	request = request.WithContext(ctx)

	// RateLimit middleware should return immediately if context is canceled
	// but it calls next.ServeHTTP if not limited.
	// Actually, the middleware handles context check before redis call.
	handler.ServeHTTP(recorder, request)
	
	// If context is canceled, redis call will return error, and middleware returns without writing 200
	// But in our case, the error handler (if any) or just the fact that next wasn't called.
	// We expect No 200 OK.
	assert.NotEqual(t, http.StatusOK, recorder.Code)
}

func TestTokenBucketRateLimitExceeded(t *testing.T) {
	server, client := setupRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	middleware, err := RateLimit(client, 1, time.Minute, WithAlgorithm(TokenBucket))
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler := middleware(next)

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "198.51.100.8:1234"

		c := NewContext(recorder, request)
		ctx := context.WithValue(request.Context(), astraContextKey, c)
		request = request.WithContext(ctx)

		handler.ServeHTTP(recorder, request)

		if attempt == 0 {
			assert.Equal(t, http.StatusOK, recorder.Code)
			continue
		}

		assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
		assert.NotEmpty(t, recorder.Header().Get("Retry-After"))

		var body map[string]any
		err := json.Unmarshal(recorder.Body.Bytes(), &body)
		require.NoError(t, err)
		assert.Equal(t, "rate_limit_exceeded", body["error"])
	}
}
