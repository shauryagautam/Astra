package auth

import (
	"context"
	"fmt"
	"github.com/astraframework/astra/json"
	nethttp "net/http"

	"github.com/astraframework/astra/resilience"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthUser represents a standardized user profile from an OAuth provider.
type OAuthUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
}

// OAuthManager handles OAuth2 flows for multiple providers.
type OAuthManager struct {
	configs map[string]*oauth2.Config
	cb      *resilience.CircuitBreaker
}

// NewOAuthManager creates a new OAuth manager.
func NewOAuthManager(providers map[string]struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}) *OAuthManager {
	configs := make(map[string]*oauth2.Config)

	if p, ok := providers["google"]; ok {
		configs["google"] = &oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		}
	}

	if p, ok := providers["github"]; ok {
		configs["github"] = &oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Endpoint:     github.Endpoint,
			Scopes:       []string{"user:email", "read:user"},
		}
	}

	return &OAuthManager{
		configs: configs,
		cb:      resilience.NewCircuitBreaker("auth:oauth"),
	}
}

// GetAuthURL returns the URL to redirect the user to for authentication.
func (m *OAuthManager) GetAuthURL(provider string, state string) (string, error) {
	cfg, ok := m.configs[provider]
	if !ok {
		return "", fmt.Errorf("oauth: provider %s not configured", provider)
	}
	return cfg.AuthCodeURL(state), nil
}

// Exchange exchanges an authorization code for a user profile.
func (m *OAuthManager) Exchange(ctx context.Context, provider string, code string) (*OAuthUser, error) {
	var user *OAuthUser
	err := m.cb.Execute(ctx, func() error {
		cfg, ok := m.configs[provider]
		if !ok {
			return fmt.Errorf("oauth: provider %s not configured", provider)
		}

		token, err := cfg.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("oauth: code exchange failed: %w", err)
		}

		client := cfg.Client(ctx, token)
		var innerErr error
		switch provider {
		case "google":
			user, innerErr = m.fetchGoogleUser(client)
		case "github":
			user, innerErr = m.fetchGithubUser(client)
		default:
			innerErr = fmt.Errorf("oauth: provider %s not supported for profile fetching", provider)
		}
		return innerErr
	})

	return user, err
}

func (m *OAuthManager) fetchGoogleUser(client *nethttp.Client) (*OAuthUser, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var profile struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &OAuthUser{
		ID:        profile.ID,
		Email:     profile.Email,
		Name:      profile.Name,
		AvatarURL: profile.Picture,
		Provider:  "google",
	}, nil
}

func (m *OAuthManager) fetchGithubUser(client *nethttp.Client) (*OAuthUser, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var profile struct {
		ID        int    `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &OAuthUser{
		ID:        fmt.Sprintf("%d", profile.ID),
		Email:     profile.Email,
		Name:      profile.Name,
		AvatarURL: profile.AvatarURL,
		Provider:  "github",
	}, nil
}
