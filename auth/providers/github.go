package providers

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/auth"
)

// NewGitHub creates a new GitHub OAuth2 provider.
//
// Usage:
//
//	github := providers.NewGitHub(
//	    os.Getenv("GITHUB_CLIENT_ID"),
//	    os.Getenv("GITHUB_CLIENT_SECRET"),
//	    "http://localhost:8080/auth/github/callback",
//	)
//	oauthManager.Register(github)
func NewGitHub(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &githubProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"read:user", "user:email"},
				AuthURL:      "https://github.com/login/oauth/authorize",
				TokenURL:     "https://github.com/login/oauth/access_token",
				UserInfoURL:  "https://api.github.com/user",
			},
		},
	}
}

type githubProvider struct {
	auth.BaseOAuth2Provider
}

func (p *githubProvider) Name() string { return "github" }

func (p *githubProvider) UserInfo(ctx context.Context, token *auth.OAuth2Token) (*auth.OAuth2User, error) {
	data, err := auth.FetchUserInfo(ctx, p.Config.UserInfoURL, token)
	if err != nil {
		return nil, err
	}

	// GitHub uses "login" for username and numeric "id"
	return &auth.OAuth2User{
		ProviderID: fmt.Sprintf("%.0f", data["id"]),
		Provider:   "github",
		Email:      strVal(data, "email"),
		Name:       strVal(data, "name"),
		AvatarURL:  strVal(data, "avatar_url"),
		Raw:        data,
	}, nil
}
