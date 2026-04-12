package authproviders

import (
	"github.com/shauryagautam/Astra/pkg/identity/auth"
	"context"
	"fmt"
)

// NewGitHub creates a new GitHub OAuth2 provider.
func NewGitHub(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &githubProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{ // #nosec G101
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"user:email", "read:user"},
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
	data, err := auth.FetchUserInfo(ctx, p.BaseOAuth2Provider.Config.UserInfoURL, token)
	if err != nil {
		return nil, err
	}

	return &auth.OAuth2User{
		ProviderID: fmt.Sprintf("%v", data["id"]),
		Provider:   "github",
		Email:      fmt.Sprintf("%v", data["email"]),
		Name:       fmt.Sprintf("%v", data["name"]),
		AvatarURL:  fmt.Sprintf("%v", data["avatar_url"]),
		Raw:        data,
	}, nil
}
