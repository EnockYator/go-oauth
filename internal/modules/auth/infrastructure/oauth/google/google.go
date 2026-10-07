// Package google implements the oauth.Provider port for Google's OIDC
// endpoints.
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"golang.org/x/oauth2"
)

const (
	authURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL    = "https://oauth2.googleapis.com/token"
	userInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// Config is the per-provider configuration for Google.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Provider implements the oauth.Provider interface for Google.
type Provider struct {
	cfg        *oauth2.Config
	httpClient *http.Client
}

// New constructs a Google provider. The httpClient is used for both the
// token exchange and the userinfo request, ensuring a shared timeout and
// connection pool.
func New(cfg Config, httpClient *http.Client) *Provider {
	return &Provider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
		},
		httpClient: httpClient,
	}
}

func (p *Provider) ID() auth.ProviderID { return auth.ProviderGoogle }

func (p *Provider) AuthCodeURL(
	state string,
	opts ...oauth2.AuthCodeOption,
) string {
	return p.cfg.AuthCodeURL(state, opts...)
}

func (p *Provider) Exchange(
	ctx context.Context,
	code string,
	opts ...oauth2.AuthCodeOption,
) (*oauth2.Token, error) {
	// oauth2 reads the HTTP client from this context key.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	return p.cfg.Exchange(ctx, code, opts...)
}

// userInfoResponse is Google's OIDC /userinfo payload (relevant fields only).
type userInfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (p *Provider) FetchUserInfo(
	ctx context.Context,
	token *oauth2.Token,
) (*auth.UserInfo, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	client := p.cfg.Client(ctx, token)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, userInfoURL, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("google: build userinfo request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: call userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"google: userinfo returned status %d", resp.StatusCode,
		)
	}

	var body userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("google: decode userinfo: %w", err)
	}

	if body.Sub == "" {
		return nil, fmt.Errorf("google: userinfo missing 'sub'")
	}
	if body.Email == "" {
		return nil, fmt.Errorf("google: userinfo missing 'email'")
	}
	// Reject unverified emails: an attacker who controls an unverified
	// Google address could otherwise impersonate a user.
	if !body.EmailVerified {
		return nil, fmt.Errorf("google: email is not verified")
	}

	return &auth.UserInfo{
		Provider:        auth.ProviderGoogle,
		ProviderSubject: body.Sub,
		Email:           body.Email,
		Name:            body.Name,
		AvatarURL:       body.Picture,
	}, nil
}
