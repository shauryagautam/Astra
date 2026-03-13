package auth

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRequestContext struct {
	req    *nethttp.Request
	claims *AuthClaims
	cookie *nethttp.Cookie
}

func (m *mockRequestContext) GetRequest() *nethttp.Request     { return m.req }
func (m *mockRequestContext) SetAuthUser(claims *AuthClaims)   { m.claims = claims }
func (m *mockRequestContext) SetCookie(cookie *nethttp.Cookie) { m.cookie = cookie }
func (m *mockRequestContext) RegenerateSession() error         { return nil }

func TestPassword(t *testing.T) {
	password := "secret123"
	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	assert.True(t, CheckPasswordHash(password, hash))
	assert.False(t, CheckPasswordHash("wrong", hash))

	assert.True(t, SecureCompare("abc", "abc"))
	assert.False(t, SecureCompare("abc", "def"))
}

func TestJWTManager(t *testing.T) {
	cfg := config.AuthConfig{
		JWTSecret:          "01234567890123456789012345678901",
		JWTIssuer:          "astra",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
	manager := NewJWTManager(cfg, nil)

	ctx := context.Background()
	userID := "user-123"

	pair, err := manager.IssueTokenPair(ctx, userID, map[string]any{"role": "admin"})
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	// Verify Access Token
	claims, err := manager.Verify(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, "admin", claims.Claims["role"])

	// Verify Refresh Token (should fail for Verify call as it's not an access token)
	_, err = manager.Verify(pair.RefreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refresh token cannot be used as access token")

	// Refresh
	newPair, err := manager.Refresh(ctx, pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, pair.AccessToken, newPair.AccessToken)
}

type mockSessionDriver struct {
	sessions map[string]map[string]any
}

func (m *mockSessionDriver) Get(ctx context.Context, id string) (map[string]any, error) {
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, assert.AnError
}

func (m *mockSessionDriver) Set(ctx context.Context, id string, data map[string]any, ttl time.Duration) error {
	m.sessions[id] = data
	return nil
}

func (m *mockSessionDriver) Destroy(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func TestJWTGuard(t *testing.T) {
	cfg := config.AuthConfig{
		JWTSecret:          "01234567890123456789012345678901",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
	manager := NewJWTManager(cfg, nil)
	guard := &JWTGuard{Manager: manager}

	t.Run("Valid Token", func(t *testing.T) {
		pair, _ := manager.IssueTokenPair(context.Background(), "user-1", nil)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		c := &mockRequestContext{req: req}

		err := guard.Attempt(c)
		require.NoError(t, err)
		require.NotNil(t, c.claims, "claims should not be nil")
		assert.Equal(t, "user-1", c.claims.UserID)
	})

	t.Run("Missing Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		c := &mockRequestContext{req: req}

		err := guard.Attempt(c)
		assert.Error(t, err)
	})
}

func TestCookieGuard(t *testing.T) {
	mock := &mockSessionDriver{sessions: make(map[string]map[string]any)}
	guard := NewCookieGuard(mock)

	t.Run("Login and Attempt", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/login", nil)
		c := &mockRequestContext{req: req}

		err := guard.Login(c, "user-2")
		require.NoError(t, err)

		// Get cookie from context
		sessionCookie := c.cookie
		require.NotNil(t, sessionCookie)

		// New request with cookie
		req2 := httptest.NewRequest("GET", "/", nil)
		req2.AddCookie(sessionCookie)
		c2 := &mockRequestContext{req: req2}

		err = guard.Attempt(c2)
		require.NoError(t, err)
		assert.Equal(t, "user-2", c2.claims.UserID)
	})

	t.Run("Logout", func(t *testing.T) {
		// Setup session
		token := "test-token"
		mock.sessions[token] = map[string]any{"userID": "user-2"}

		req := httptest.NewRequest("POST", "/logout", nil)
		// Use real cookie
		req.AddCookie(&nethttp.Cookie{Name: guard.CookieName, Value: token})

		c := &mockRequestContext{req: req}
		err := guard.Logout(c)
		require.NoError(t, err)
		assert.NotContains(t, mock.sessions, token)
	})
}

func TestOAuthManager(t *testing.T) {
	providers := map[string]struct {
		ClientID     string
		ClientSecret string
		RedirectURL  string
	}{
		"google": {ClientID: "id", ClientSecret: "secret", RedirectURL: "http://callback"},
	}

	manager := NewOAuthManager(providers)

	t.Run("GetAuthURL", func(t *testing.T) {
		url, err := manager.GetAuthURL("google", "state123")
		require.NoError(t, err)
		assert.Contains(t, url, "client_id=id")
		assert.Contains(t, url, "state=state123")
		assert.Contains(t, url, "redirect_uri=http%3A%2F%2Fcallback")
	})

	t.Run("Unsupported Provider", func(t *testing.T) {
		_, err := manager.GetAuthURL("github", "state")
		assert.Error(t, err)
	})
}

func TestJWTRotation(t *testing.T) {
	// Old configuration with a single secret
	oldSecret := "old-secret-key-32-bytes-minimum"
	oldCfg := config.AuthConfig{
		JWTSecret:          oldSecret,
		JWTIssuer:          "astra",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * time.Hour,
	}
	oldManager := NewJWTManager(oldCfg, nil)

	// Issue token with old manager
	ctx := context.Background()
	pair, err := oldManager.IssueTokenPair(ctx, "user-1", nil)
	require.NoError(t, err)

	// New configuration with rotated secrets (new active secret, old secret kept for verification)
	newSecret := "new-secret-key-32-bytes-minimum"
	// Optional key ids: "kid1:new-secret..., kid2:old-secret..." or just hashed automatically
	newCfg := config.AuthConfig{
		JWTSecret:          newSecret + "," + oldSecret,
		JWTIssuer:          "astra",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * time.Hour,
	}
	newManager := NewJWTManager(newCfg, nil)

	// New manager should be able to verify the old token
	claims, err := newManager.Verify(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)

	// New manager issues token with the new active secret
	newPair, err := newManager.IssueTokenPair(ctx, "user-2", nil)
	require.NoError(t, err)

	claims2, err := newManager.Verify(newPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-2", claims2.UserID)

	// Old manager should FAIL to verify the new token since it doesn't know the new secret
	_, err = oldManager.Verify(newPair.AccessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")

	// Explicit KID configs
	explicitCfg := config.AuthConfig{
		JWTSecret:          "v2:new-secret-key-32-bytes-minimum,v1:old-secret-key-32-bytes-minimum",
		JWTIssuer:          "astra",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * time.Hour,
	}
	explicitManager := NewJWTManager(explicitCfg, nil)
	assert.Equal(t, "v2", explicitManager.activeKeyID)

	explicitPair, err := explicitManager.IssueTokenPair(ctx, "user-3", nil)
	require.NoError(t, err)
	claims3, err := explicitManager.Verify(explicitPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-3", claims3.UserID)
}
