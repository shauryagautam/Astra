package runtime

import (
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/identity/auth"
	"github.com/shauryagautam/Astra/pkg/identity/auth/providers"
	"github.com/redis/go-redis/v9"
)

// ProvideOAuth2Manager initializes OAuth2Manager with providers from config.
func ProvideOAuth2Manager(cfg *config.AstraConfig, redisClient redis.UniversalClient) *auth.OAuth2Manager {
	m := auth.NewOAuth2Manager(redisClient)

	// Google
	if cfg.OAuth2.Google.ClientID != "" {
		m.Register(authproviders.NewGoogle(
			cfg.OAuth2.Google.ClientID,
			cfg.OAuth2.Google.ClientSecret,
			cfg.OAuth2.Google.RedirectURL,
		))
	}

	// GitHub
	if cfg.OAuth2.GitHub.ClientID != "" {
		m.Register(authproviders.NewGitHub(
			cfg.OAuth2.GitHub.ClientID,
			cfg.OAuth2.GitHub.ClientSecret,
			cfg.OAuth2.GitHub.RedirectURL,
		))
	}

	// Discord
	if cfg.OAuth2.Discord.ClientID != "" {
		m.Register(authproviders.NewDiscord(
			cfg.OAuth2.Discord.ClientID,
			cfg.OAuth2.Discord.ClientSecret,
			cfg.OAuth2.Discord.RedirectURL,
		))
	}

	// Apple
	if cfg.OAuth2.Apple.ClientID != "" {
		m.Register(authproviders.NewApple(
			cfg.OAuth2.Apple.ClientID,
			cfg.OAuth2.Apple.ClientSecret,
			cfg.OAuth2.Apple.RedirectURL,
		))
	}

	// Microsoft
	if cfg.OAuth2.Microsoft.ClientID != "" {
		m.Register(authproviders.NewMicrosoft(
			cfg.OAuth2.Microsoft.ClientID,
			cfg.OAuth2.Microsoft.ClientSecret,
			cfg.OAuth2.Microsoft.RedirectURL,
		))
	}

	return m
}
