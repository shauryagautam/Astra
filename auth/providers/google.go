package providers

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/auth"
)

// NewGoogle creates a new Google OAuth2 provider.
//
// Usage:
//
//	google := providers.NewGoogle(
//	    os.Getenv("GOOGLE_CLIENT_ID"),
//	    os.Getenv("GOOGLE_CLIENT_SECRET"),
//	    "http://localhost:8080/auth/google/callback",
//	)
//	oauthManager.Register(google)
func NewGoogle(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &googleProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{ // #nosec G101
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"openid", "email", "profile"},
				AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:     "https://oauth2.googleapis.com/token",
				UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
			},
		},
	}
}

type googleProvider struct {
	auth.BaseOAuth2Provider
}

func (p *googleProvider) Name() string { return "google" }

func (p *googleProvider) UserInfo(ctx context.Context, token *auth.OAuth2Token) (*auth.OAuth2User, error) {
	data, err := auth.FetchUserInfo(ctx, p.Config.UserInfoURL, token)
	if err != nil {
		return nil, err
	}

	return &auth.OAuth2User{
		ProviderID: fmt.Sprintf("%v", data["id"]),
		Provider:   "google",
		Email:      strVal(data, "email"),
		Name:       strVal(data, "name"),
		AvatarURL:  strVal(data, "picture"),
		Raw:        data,
	}, nil
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
