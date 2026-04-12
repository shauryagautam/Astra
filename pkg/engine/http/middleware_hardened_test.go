package http // Astra Hardened Middleware Tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	identityclaims "github.com/shauryagautam/Astra/pkg/identity/claims"
)

func TestCSRF_Hardened_SPA_Defaults(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := CSRF(false, WithCSRFMode(CSRFModeSPA))(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	c := NewContext(w, req)
	ctx := context.WithValue(req.Context(), astraContextKey, c)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Check if SPA defaults were applied
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, ck := range cookies {
		if ck.Name == "XSRF-TOKEN" {
			csrfCookie = ck
			break
		}
	}

	require.NotNil(t, csrfCookie, "SPA mode should use XSRF-TOKEN cookie name by default")
	assert.False(t, csrfCookie.HttpOnly, "SPA mode cookie should NOT be HttpOnly for reflection")
}

func TestCSRF_Hardened_API_Stateless(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := CSRF(false, WithCSRFMode(CSRFModeAPI))(next)

	// Case 1: Authenticated with JWT (stateless)
	req1 := httptest.NewRequest("POST", "/api/data", nil)
	req1.Header.Set("Authorization", "Bearer valid.jwt.token")
	w1 := httptest.NewRecorder()
	
	c1 := NewContext(w1, req1)
	c1.SetAuthUser(&identityclaims.AuthClaims{UserID: "user-123"})
	ctx1 := context.WithValue(req1.Context(), astraContextKey, c1)
	req1 = req1.WithContext(ctx1)

	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "ok", w1.Body.String())

	// Case 2: Unauthenticated API request
	req2 := httptest.NewRequest("POST", "/api/data", nil)
	w2 := httptest.NewRecorder()
	
	c2 := NewContext(w2, req2)
	ctx2 := context.WithValue(req2.Context(), astraContextKey, c2)
	req2 = req2.WithContext(ctx2)

	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}

func TestIPExtraction_Hardened(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.1.1/32"),
	}

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
	}{
		{
			name:       "Direct connection from untrusted IP",
			remoteAddr: "1.2.3.4:1234",
			xff:        "",
			expected:   "1.2.3.4",
		},
		{
			name:       "Connection through trusted proxy, single XFF",
			remoteAddr: "10.0.0.1:1234",
			xff:        "1.2.3.4",
			expected:   "1.2.3.4",
		},
		{
			name:       "Connection through trusted proxy, multiple XFF (spoofed)",
			remoteAddr: "10.0.0.1:1234",
			xff:        "8.8.8.8, 1.2.3.4",
			expected:   "1.2.3.4",
		},
		{
			name:       "Multiple trusted proxies in chain",
			remoteAddr: "10.0.0.1:1234",
			xff:        "1.2.3.4, 10.0.0.2, 192.168.1.1",
			expected:   "1.2.3.4",
		},
		{
			name:       "Untrusted proxy in middle of chain (spoofed)",
			remoteAddr: "10.0.0.1:1234",
			xff:        "1.1.1.1, 5.5.5.5, 10.0.0.2",
			expected:   "5.5.5.5", // 5.5.5.5 is the first untrusted IP from the right
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			ip := GetClientIP(req, trusted)
			assert.Equal(t, tt.expected, ip)
		})
	}
}
