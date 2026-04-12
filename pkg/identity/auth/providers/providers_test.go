package authproviders

import (
	"github.com/shauryagautam/Astra/pkg/identity/auth"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoogleProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer mock-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "123456789",
			"email":   "test@google.com",
			"name":    "Google User",
			"picture": "http://avatar.url",
		})
	}))
	defer server.Close()

	p := NewGoogle("client-id", "client-secret", "http://callback")
	assert.Equal(t, "google", p.Name())

	// Override UserInfoURL for testing
	gp := p.(*googleProvider)
	gp.Config.UserInfoURL = server.URL

	user, err := p.UserInfo(context.Background(), &auth.OAuth2Token{AccessToken: "mock-token"})
	assert.NoError(t, err)
	assert.Equal(t, "123456789", user.ProviderID)
	assert.Equal(t, "google", user.Provider)
	assert.Equal(t, "test@google.com", user.Email)
	assert.Equal(t, "Google User", user.Name)
	assert.Equal(t, "http://avatar.url", user.AvatarURL)
}

func TestAppleProvider(t *testing.T) {
	p := NewApple("client-id", "client-secret", "http://callback")
	assert.Equal(t, "apple", p.Name())

	// Mock IDToken for testing. Only payload is parsed in ParseUnverified.
	payload := `{"sub": "apple-123", "email": "test@apple.com"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mockIDToken := "header." + encodedPayload + ".signature"

	user, err := p.UserInfo(context.Background(), &auth.OAuth2Token{AccessToken: "mock-token", IDToken: mockIDToken})
	assert.NoError(t, err)
	assert.Equal(t, "apple-123", user.ProviderID)
	assert.Equal(t, "apple", user.Provider)
	assert.Equal(t, "test@apple.com", user.Email)
}

func TestMicrosoftProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer mock-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":          "ms-123",
			"mail":        "test@microsoft.com",
			"displayName": "MS User",
		})
	}))
	defer server.Close()

	p := NewMicrosoft("client-id", "client-secret", "http://callback")
	assert.Equal(t, "microsoft", p.Name())

	mp := p.(*microsoftProvider)
	mp.Config.UserInfoURL = server.URL

	user, err := p.UserInfo(context.Background(), &auth.OAuth2Token{AccessToken: "mock-token"})
	assert.NoError(t, err)
	assert.Equal(t, "ms-123", user.ProviderID)
	assert.Equal(t, "microsoft", user.Provider)
	assert.Equal(t, "test@microsoft.com", user.Email)
	assert.Equal(t, "MS User", user.Name)
}
func TestGitHubProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer mock-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":         12345,
			"email":      "test@github.com",
			"name":       "GitHub User",
			"avatar_url": "http://github.avatar",
		})
	}))
	defer server.Close()

	p := NewGitHub("client-id", "client-secret", "http://callback")
	assert.Equal(t, "github", p.Name())

	gp := p.(*githubProvider)
	gp.Config.UserInfoURL = server.URL

	user, err := p.UserInfo(context.Background(), &auth.OAuth2Token{AccessToken: "mock-token"})
	assert.NoError(t, err)
	assert.Equal(t, "12345", user.ProviderID)
	assert.Equal(t, "github", user.Provider)
	assert.Equal(t, "test@github.com", user.Email)
}

func TestDiscordProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer mock-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":       "discord-123",
			"email":    "test@discord.com",
			"username": "DiscordUser",
			"avatar":   "avatar-hash",
		})
	}))
	defer server.Close()

	p := NewDiscord("client-id", "client-secret", "http://callback")
	assert.Equal(t, "discord", p.Name())

	dp := p.(*discordProvider)
	dp.Config.UserInfoURL = server.URL

	user, err := p.UserInfo(context.Background(), &auth.OAuth2Token{AccessToken: "mock-token"})
	assert.NoError(t, err)
	assert.Equal(t, "discord-123", user.ProviderID)
	assert.Equal(t, "discord", user.Provider)
	assert.Equal(t, "test@discord.com", user.Email)
}
