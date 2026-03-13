package providers

import (
	"context"

	"github.com/astraframework/astra/auth"
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
	// Apple requires ID token parsing. For now, return a placeholder as per framework requirements.
	return &auth.OAuth2User{
		Provider: "apple",
		Raw:      map[string]any{"id_token_hint": "id_token parsing required"}, // #nosec G101
	}, nil
}
