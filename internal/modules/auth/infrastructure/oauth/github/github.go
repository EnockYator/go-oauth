// Package github implements the oauth.Provider port for GitHub's OAuth2
// endpoints.
//
// GitHub is not an OIDC provider and has no /userinfo endpoint: the user
// profile is fetched from /user and, when the public email is hidden,
// /user/emails.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"golang.org/x/oauth2"
)

const (
	authURL  = "https://github.com/login/oauth/authorize"
	tokenURL = "https://github.com/login/oauth/access_token"
	userURL  = "https://api.github.com/user"
	emailURL = "https://api.github.com/user/emails"

	// userAgent is required by the GitHub API; requests without it are
	// rejected with 403.
	userAgent = "go-oauth"

	acceptHeader = "application/vnd.github+json"
	apiVersion   = "2022-11-28"
)

// Config is the per-provider configuration for GitHub.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Provider implements the oauth.Provider interface for GitHub.
type Provider struct {
	cfg        *oauth2.Config
	httpClient *http.Client
}

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

func (p *Provider) ID() auth.ProviderID { return auth.ProviderGitHub }

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
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	return p.cfg.Exchange(ctx, code, opts...)
}

// userResponse is the subset of GET /user we care about.
type userResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type emailResponse struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (p *Provider) FetchUserInfo(
	ctx context.Context,
	token *oauth2.Token,
) (*auth.UserInfo, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	client := p.cfg.Client(ctx, token)

	user, err := p.fetchUser(ctx, client)
	if err != nil {
		return nil, err
	}

	// GitHub omits `email` on /user when the user has hidden it. Fall back
	// to /user/emails, which exposes the primary verified address.
	email := user.Email
	if email == "" {
		email, err = p.fetchPrimaryEmail(ctx, client)
		if err != nil {
			return nil, err
		}
	}

	if user.ID == 0 {
		return nil, fmt.Errorf("github: user response missing id")
	}
	if email == "" {
		return nil, fmt.Errorf("github: no verified email available")
	}

	return &auth.UserInfo{
		Provider: auth.ProviderGitHub,
		// GitHub's numeric ID is the stable identifier; `login` can be
		// renamed by the user.
		ProviderSubject: fmt.Sprintf("%d", user.ID),
		Email:           email,
		Name:            displayName(user),
		AvatarURL:       user.AvatarURL,
	}, nil
}

func (p *Provider) fetchUser(
	ctx context.Context,
	client *http.Client,
) (*userResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build /user request: %w", err)
	}
	setGitHubHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: call /user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"github: /user returned status %d", resp.StatusCode,
		)
	}

	var body userResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("github: decode /user: %w", err)
	}
	return &body, nil
}

func (p *Provider) fetchPrimaryEmail(
	ctx context.Context,
	client *http.Client,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, emailURL, nil)
	if err != nil {
		return "", fmt.Errorf("github: build /user/emails request: %w", err)
	}
	setGitHubHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github: call /user/emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"github: /user/emails returned status %d", resp.StatusCode,
		)
	}

	var emails []emailResponse
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("github: decode /user/emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	// Fall back to any verified email rather than failing outright.
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("github: no verified email found")
}

func setGitHubHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
}

// displayName prefers the user's full name; falls back to the login handle,
// which GitHub always returns.
func displayName(u *userResponse) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Login
}
