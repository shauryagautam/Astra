package providers

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/auth"
)

// NewMicrosoft creates a new Microsoft OAuth2 provider.
func NewMicrosoft(clientID, clientSecret, redirectURL string) auth.OAuth2Provider {
	return &microsoftProvider{
		BaseOAuth2Provider: auth.BaseOAuth2Provider{
			Config: auth.OAuth2ProviderConfig{ // #nosec G101
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"openid", "profile", "email", "User.Read"},
				AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				UserInfoURL:  "https://graph.microsoft.com/v1.0/me",
			},
		},
	}
}

type microsoftProvider struct {
	auth.BaseOAuth2Provider
}

func (p *microsoftProvider) Name() string { return "microsoft" }

func (p *microsoftProvider) UserInfo(ctx context.Context, token *auth.OAuth2Token) (*auth.OAuth2User, error) {
	data, err := auth.FetchUserInfo(ctx, p.Config.UserInfoURL, token)
	if err != nil {
		return nil, err
	}

	email, _ := data["mail"].(string)
	if email == "" {
		email, _ = data["userPrincipalName"].(string)
	}

	return &auth.OAuth2User{
		ProviderID: fmt.Sprintf("%v", data["id"]),
		Provider:   "microsoft",
		Email:      email,
		Name:       fmt.Sprintf("%v", data["displayName"]),
		Raw:        data,
	}, nil
}
