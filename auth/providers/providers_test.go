package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraframework/astra/auth"
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

	user, err := p.UserInfo(context.Background(), &auth.OAuth2Token{AccessToken: "mock-token"})
	assert.NoError(t, err)
	assert.Equal(t, "apple", user.Provider)
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
