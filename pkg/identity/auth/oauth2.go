package auth // Identity authentication and OAuth2 core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/shauryagautam/Astra/pkg/engine/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// OAuth2User represents the user information returned by an OAuth2 provider.
type OAuth2User struct {
	ProviderID string         `json:"provider_id"` // Unique ID from the provider
	Provider   string         `json:"provider"`    // Provider name (google, github, discord)
	Email      string         `json:"email"`
	Name       string         `json:"name"`
	AvatarURL  string         `json:"avatar_url"`
	Raw        map[string]any `json:"raw"` // Full raw response from the provider
}

// OAuth2Config holds the configuration for a single OAuth2 provider.
type OAuth2ProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
}

// OAuth2Token represents the token response from an OAuth2 provider.
type OAuth2Token struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// OAuth2Provider defines the interface for OAuth2 providers.
type OAuth2Provider interface {
	// Name returns the provider name (e.g., "google", "github").
	Name() string

	// AuthURL returns the URL to redirect the user to for authentication.
	// The state parameter should be stored and verified on callback.
	// Opts is an optional map for PKCE parameters (code_challenge, code_challenge_method).
	AuthURL(state string, opts ...map[string]string) string

	// Exchange exchanges an authorization code for an access token.
	// Opts is an optional map for PKCE parameters (code_verifier).
	Exchange(ctx context.Context, code string, opts ...map[string]string) (*OAuth2Token, error)

	// UserInfo retrieves user information using the access token.
	UserInfo(ctx context.Context, token *OAuth2Token) (*OAuth2User, error)
}

// OAuth2Manager coordinates OAuth2 flows with state management via Redis.
type OAuth2Manager struct {
	providers map[string]OAuth2Provider
	redis     redis.UniversalClient
}

// NewOAuth2Manager creates a new OAuth2Manager.
func NewOAuth2Manager(redisClient redis.UniversalClient) *OAuth2Manager {
	return &OAuth2Manager{
		providers: make(map[string]OAuth2Provider),
		redis:     redisClient,
	}
}

// Register adds a provider to the manager.
func (m *OAuth2Manager) Register(provider OAuth2Provider) {
	m.providers[provider.Name()] = provider
}

// Provider returns a registered provider by name.
func (m *OAuth2Manager) Provider(name string) (OAuth2Provider, error) {
	p, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("oauth2: unknown provider %q", name)
	}
	return p, nil
}

// GenerateState creates a cryptographically random state parameter and stores it in Redis.
func (m *OAuth2Manager) GenerateState(ctx context.Context) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth2: failed to generate state: %w", err)
	}
	state := hex.EncodeToString(b)

	if m.redis != nil {
		key := "oauth2:state:" + state
		if err := m.redis.Set(ctx, key, "1", 10*time.Minute).Err(); err != nil {
			return "", fmt.Errorf("oauth2: failed to store state: %w", err)
		}
	}

	return state, nil
}

// VerifyState checks that the state parameter is valid and has not been used before.
func (m *OAuth2Manager) VerifyState(ctx context.Context, state string) error {
	if state == "" {
		return fmt.Errorf("oauth2: state parameter is empty")
	}

	if m.redis != nil {
		key := "oauth2:state:" + state
		val, err := m.redis.Get(ctx, key).Result()
		if err != nil || val != "1" {
			return fmt.Errorf("oauth2: invalid or expired state parameter")
		}
		// Delete after use (one-time use)
		m.redis.Del(ctx, key)
	}

	return nil
}

// ─── Base Provider Implementation ─────────────────────────────────────

// BaseOAuth2Provider implements common OAuth2 exchange logic.
// Provider implementations in sub-packages embed this for code reuse.
type BaseOAuth2Provider struct {
	Config OAuth2ProviderConfig
}

// AuthURL builds the standard OAuth2 authorization URL.
// It supports optional PKCE parameters (code_challenge).
func (p *BaseOAuth2Provider) AuthURL(state string, opts ...map[string]string) string {
	params := url.Values{
		"client_id":     {p.Config.ClientID},
		"redirect_uri":  {p.Config.RedirectURL},
		"response_type": {"code"},
		"state":         {state},
		"scope":         {strings.Join(p.Config.Scopes, " ")},
	}

	if len(opts) > 0 {
		for k, v := range opts[0] {
			params.Set(k, v)
		}
	}

	return p.Config.AuthURL + "?" + params.Encode()
}

// Exchange exchanges an authorization code for a token using standard OAuth2 flow.
// It supports optional PKCE parameters (code_verifier).
func (p *BaseOAuth2Provider) Exchange(ctx context.Context, code string, opts ...map[string]string) (*OAuth2Token, error) {
	data := url.Values{
		"client_id":     {p.Config.ClientID},
		"client_secret": {p.Config.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.Config.RedirectURL},
		"grant_type":    {"authorization_code"},
	}

	if len(opts) > 0 {
		for k, v := range opts[0] {
			data.Set(k, v)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.Config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth2: token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth2: token exchange returned %d: %s", resp.StatusCode, string(body))
	}

	var token OAuth2Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("oauth2: failed to parse token response: %w", err)
	}
	return &token, nil
}

// FetchUserInfo retrieves user information from a provider's userinfo endpoint.
func FetchUserInfo(ctx context.Context, url string, token *OAuth2Token) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth2: userinfo returned %d: %s", resp.StatusCode, string(body))
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("oauth2: failed to decode userinfo response: %w (body: %s)", err, string(body))
	}
	return data, nil
}

// ─── PKCE Helpers ─────────────────────────────────────────────────────

// GenerateCodeVerifier creates a cryptographically random string for PKCE.
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge creates an S256 challenge from a code verifier.
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

