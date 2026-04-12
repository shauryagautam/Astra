package authproviders // Astra Apple OAuth2 Provider [Final Sync]

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/shauryagautam/Astra/pkg/engine/json"
	"github.com/shauryagautam/Astra/pkg/identity/auth"
)

// NewApple creates a new Apple OAuth2 provider.
func NewApple(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &appleProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"name", "email"},
				AuthURL:      "https://appleid.apple.com/auth/authorize",
				TokenURL:     "https://appleid.apple.com/auth/token",
			},
		},
	}
}

type appleProvider struct {
	auth.BaseOAuth2Provider
}

func (p *appleProvider) Name() string { return "apple" }

func (p *appleProvider) UserInfo(ctx context.Context, token *auth.OAuth2Token) (*auth.OAuth2User, error) {
	if token.IDToken == "" {
		return &auth.OAuth2User{
			Provider: "apple",
			Raw:      map[string]any{"error": "id_token missing"},
		}, nil
	}

	// Manual parsing of ID token payload (JWT) to avoid dependency issues/panics
	parts := strings.Split(token.IDToken, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("apple: invalid id_token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("apple: failed to decode id_token payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("apple: failed to unmarshal id_token claims: %w", err)
	}

	return &auth.OAuth2User{
		ProviderID: fmt.Sprintf("%v", claims["sub"]),
		Provider:   "apple",
		Email:      fmt.Sprintf("%v", claims["email"]),
		Raw:        claims,
	}, nil
}
