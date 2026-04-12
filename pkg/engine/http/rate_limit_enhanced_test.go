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

func setupEnhancedRateLimitClient(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	return server, client
}

func TestRateLimitEnhanced_IPSpoofingProtection(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	// Configure trusted proxies
	middleware, err := RateLimit(client, 2, time.Minute,
		WithTrustedProxies("10.0.0.0/8", "192.168.0.0/16"),
		WithIPSpoofingProtection(true),
		WithIPHeaderValidation(true),
	)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware(next)

	tests := []struct {
		name        string
		remoteAddr  string
		headers     map[string]string
		expectedKey string
		description string
	}{
		{
			name:        "Direct connection without proxy",
			remoteAddr:  "203.0.113.10:1234",
			headers:     map[string]string{},
			expectedKey: "203.0.113.10",
			description: "Should use remote IP when no proxy headers",
		},
		{
			name:       "Trusted proxy with X-Forwarded-For",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.20, 10.0.0.2",
			},
			expectedKey: "203.0.113.20",
			description: "Should use the first untrusted IP from the right in X-Forwarded-For",
		},
		{
			name:       "Untrusted proxy ignores headers",
			remoteAddr: "203.0.113.30:1234",
			headers: map[string]string{
				"X-Forwarded-For": "192.0.2.1, 10.0.0.1",
			},
			expectedKey: "203.0.113.30",
			description: "Should ignore headers when remote proxy is not trusted",
		},
		{
			name:       "Cloudflare connecting IP",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.40",
			},
			expectedKey: "203.0.113.40",
			description: "Should use CF-Connecting-IP header from trusted proxy when XFF is missing",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.50",
			},
			expectedKey: "203.0.113.50",
			description: "Should use X-Real-IP header from trusted proxy when XFF is missing",
		},
		{
			name:       "Invalid IP in header falls back to remote",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "invalid-ip",
			},
			expectedKey: "10.0.0.1",
			description: "Should fall back to remote IP when XFF only contains invalid IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.FlushAll() // Clean Redis for each case
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			
			// Inject Astra Context manually because middleware needs it
			c := NewContext(w, req)
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			// Check that rate limiting headers are set
			assert.Equal(t, "2", w.Header().Get("X-RateLimit-Limit"))
			assert.Equal(t, "1", w.Header().Get("X-RateLimit-Remaining"))
			
			// Verify the key used in Redis contains the expected IP
			keys := server.Keys()
			require.NotEmpty(t, keys)
			assert.Contains(t, keys[0], tt.expectedKey)
		})
	}
}

func TestRateLimitEnhanced_HeaderInjectionProtection(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	middleware, err := RateLimit(client, 1, time.Minute,
		WithTrustedProxies("10.0.0.0/8"),
		WithMaxProxyDepth(3),
	)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware(next)

	// Test with untrusted IP in chain
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.2, 10.0.0.3")

	w := httptest.NewRecorder()
	
	c := NewContext(w, req)
	ctx := context.WithValue(req.Context(), astraContextKey, c)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	// Should walk back until first untrusted IP
	keys := server.Keys()
	require.NotEmpty(t, keys)
	assert.Contains(t, keys[0], "203.0.113.1")
}

func TestRateLimitEnhanced_MultipleHeadersPriority(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	middleware, err := RateLimit(client, 1, time.Minute,
		WithTrustedProxies("10.0.0.0/8"),
	)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware(next)

	tests := []struct {
		name        string
		headers     map[string]string
		expectedIP  string
		description string
	}{
		{
			name: "X-Forwarded-For takes priority over X-Real-IP",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10",
				"X-Real-IP":       "203.0.113.20",
			},
			expectedIP:  "203.0.113.10",
			description: "X-Forwarded-For should be prioritized",
		},
		{
			name: "CF-Connecting-IP takes priority",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.30",
				"X-Forwarded-For":  "203.0.113.10",
				"X-Real-IP":       "203.0.113.20",
			},
			expectedIP:  "203.0.113.30",
			description: "CF-Connecting-IP should be prioritized for Cloudflare",
		},
		{
			name: "X-Client-IP fallback",
			headers: map[string]string{
				"X-Client-IP": "203.0.113.40",
			},
			expectedIP:  "203.0.113.40",
			description: "X-Client-IP should be used as fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.FlushAll() // Clear Redis between priority tests
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.0.0.1:8080"
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			keys := server.Keys()
			require.NotEmpty(t, keys)
			assert.Contains(t, keys[0], tt.expectedIP)
		})
	}
}

func TestRateLimitEnhanced_DisabledSpoofingProtection(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	middleware, err := RateLimit(client, 1, time.Minute,
		WithIPSpoofingProtection(false),
		WithIPHeaderValidation(false),
	)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware(next)

	// With spoofing protection disabled, should use remote IP regardless of headers
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.100:1234"
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	w := httptest.NewRecorder()
	
	c := NewContext(w, req)
	ctx := context.WithValue(req.Context(), astraContextKey, c)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	keys := server.Keys()
	require.NotEmpty(t, keys)
	assert.Contains(t, keys[0], "203.0.113.100")
}

func TestRateLimitEnhanced_TrustedProxyValidation(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	middleware, err := RateLimit(client, 1, time.Minute,
		WithTrustedProxies("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"),
	)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware(next)

	tests := []struct {
		name        string
		remoteAddr  string
		headers     map[string]string
		expectedIP  string
		description string
	}{
		{
			name:       "Trusted proxy 10.0.0.0/8",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10",
			},
			expectedIP:  "203.0.113.10",
			description: "Should trust proxy in 10.0.0.0/8 range",
		},
		{
			name:       "Trusted proxy 172.16.0.0/12",
			remoteAddr: "172.16.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.20",
			},
			expectedIP:  "203.0.113.20",
			description: "Should trust proxy in 172.16.0.0/12 range",
		},
		{
			name:       "Trusted proxy 192.168.0.0/16",
			remoteAddr: "192.168.1.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.30",
			},
			expectedIP:  "203.0.113.30",
			description: "Should trust proxy in 192.168.0.0/16 range",
		},
		{
			name:       "Untrusted proxy",
			remoteAddr: "203.0.113.200:8080",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			expectedIP:  "203.0.113.200",
			description: "Should not trust proxy outside trusted ranges",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.FlushAll() // Clean Redis for each case
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			keys := server.Keys()
			require.NotEmpty(t, keys)
			assert.Contains(t, keys[0], tt.expectedIP)
		})
	}
}

func TestRateLimitEnhanced_ErrorHandling(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	middleware, err := RateLimit(client, 1, time.Minute,
		WithTrustedProxies("10.0.0.0/8"),
		WithIPSpoofingProtection(true),
	)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := middleware(next)

	// Test with malformed remote address
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "invalid-address"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	w := httptest.NewRecorder()
	
	c := NewContext(w, req)
	ctx := context.WithValue(req.Context(), astraContextKey, c)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	// Should handle gracefully and use some form of identifier
	keys := server.Keys()
	require.NotEmpty(t, keys)
}

func TestRateLimitEnhanced_AlgorithmPreservation(t *testing.T) {
	server, client := setupEnhancedRateLimitClient(t)
	defer server.Close()
	defer client.Close()

	// Clear Redis before test
	server.FlushAll()

	tests := []struct {
		name      string
		algorithm RateLimitAlgorithm
	}{
		{"Sliding Window", SlidingWindow},
		{"Token Bucket", TokenBucket},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.FlushAll() // Clean Redis between algorithm tests
			middleware, err := RateLimit(client, 1, time.Minute,
				WithAlgorithm(tt.algorithm),
				WithTrustedProxies("10.0.0.0/8"),
			)
			require.NoError(t, err)

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status":"ok"}`))
			})

			handler := middleware(next)

			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "10.0.0.1:8080"
			req.Header.Set("X-Forwarded-For", "203.0.113.10")

			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			// First request should succeed
			assert.Equal(t, http.StatusOK, w.Code)

			// Second request should be rate limited
			w2 := httptest.NewRecorder()
			c2 := NewContext(w2, req)
			ctx2 := context.WithValue(req.Context(), astraContextKey, c2)
			req = req.WithContext(ctx2)

			handler.ServeHTTP(w2, req)
			assert.Equal(t, http.StatusTooManyRequests, w2.Code)

			var body map[string]any
			err = json.Unmarshal(w2.Body.Bytes(), &body)
			require.NoError(t, err)
			assert.Equal(t, "rate_limit_exceeded", body["error"])
		})
	}
}
