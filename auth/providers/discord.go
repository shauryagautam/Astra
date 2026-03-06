package providers

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/auth"
)

// NewDiscord creates a new Discord OAuth2 provider.
//
// Usage:
//
//	discord := providers.NewDiscord(
//	    os.Getenv("DISCORD_CLIENT_ID"),
//	    os.Getenv("DISCORD_CLIENT_SECRET"),
//	    "http://localhost:8080/auth/discord/callback",
//	)
//	oauthManager.Register(discord)
func NewDiscord(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &discordProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{
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
	data, err := auth.FetchUserInfo(ctx, p.Config.UserInfoURL, token)
	if err != nil {
		return nil, err
	}

	// Build Discord avatar URL
	discordID := strVal(data, "id")
	avatar := strVal(data, "avatar")
	var avatarURL string
	if avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", discordID, avatar)
	}

	// Discord uses "username" and may have "global_name"
	name := strVal(data, "global_name")
	if name == "" {
		name = strVal(data, "username")
	}

	return &auth.OAuth2User{
		ProviderID: discordID,
		Provider:   "discord",
		Email:      strVal(data, "email"),
		Name:       name,
		AvatarURL:  avatarURL,
		Raw:        data,
	}, nil
}
