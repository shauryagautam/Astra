package authproviders

import (
	"github.com/shauryagautam/Astra/pkg/identity/auth"
	"context"
	"fmt"
)

// NewDiscord creates a new Discord OAuth2 provider.
func NewDiscord(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &discordProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{ // #nosec G101
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"identify", "email"},
				AuthURL:      "https://discord.com/api/oauth2/authorize",
				TokenURL:     "https://discord.com/api/oauth2/token",
				UserInfoURL:  "https://discord.com/api/users/@me",
			},
		},
	}
}

type discordProvider struct {
	auth.BaseOAuth2Provider
}

func (p *discordProvider) Name() string { return "discord" }

func (p *discordProvider) UserInfo(ctx context.Context, token *auth.OAuth2Token) (*auth.OAuth2User, error) {
	data, err := auth.FetchUserInfo(ctx, p.BaseOAuth2Provider.Config.UserInfoURL, token)
	if err != nil {
		return nil, err
	}

	return &auth.OAuth2User{
		ProviderID: fmt.Sprintf("%v", data["id"]),
		Provider:   "discord",
		Email:      fmt.Sprintf("%v", data["email"]),
		Name:       fmt.Sprintf("%v", data["username"]),
		AvatarURL:  fmt.Sprintf("https://cdn.discordapp.com/avatars/%v/%v.png", data["id"], data["avatar"]),
		Raw:        data,
	}, nil
}
