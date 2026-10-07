package oauth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauth/github"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauth/google"
)

// BuildConfig is the top-level configuration for the provider registry.
type BuildConfig struct {
	// HTTPClient is used for all outbound provider calls. If nil, a client
	// with a 10-second timeout is created. ALWAYS provide a client with a
	// timeout — oauth2's default is http.DefaultClient, which has none.
	HTTPClient *http.Client

	Google *ProviderConfig
	GitHub *ProviderConfig
}

// Build constructs a Registry from the supplied configuration.
//
// Only providers whose config is non-nil are registered. A nil ClientID or
// RedirectURL for a provider that IS configured is treated as a fatal
// configuration error, since silently dropping a provider in production
// leads to confusing 404s on /auth/login/{provider}.
func Build(cfg BuildConfig) (*Registry, error) {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	reg := NewRegistry()

	if cfg.Google != nil {
		if err := validateConfig(auth.ProviderGoogle, cfg.Google); err != nil {
			return nil, err
		}
		reg.Register(google.New(google.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Scopes:       cfg.Google.Scopes,
		}, httpClient))
	}

	if cfg.GitHub != nil {
		if err := validateConfig(auth.ProviderGitHub, cfg.GitHub); err != nil {
			return nil, err
		}
		reg.Register(github.New(github.Config{
			ClientID:     cfg.GitHub.ClientID,
			ClientSecret: cfg.GitHub.ClientSecret,
			RedirectURL:  cfg.GitHub.RedirectURL,
			Scopes:       cfg.GitHub.Scopes,
		}, httpClient))
	}

	if len(reg.IDs()) == 0 {
		return nil, fmt.Errorf("oauth: no providers configured")
	}

	return reg, nil
}

func validateConfig(id auth.ProviderID, cfg *ProviderConfig) error {
	if cfg.ClientID == "" {
		return fmt.Errorf("oauth: %s: client_id is required", id)
	}
	if cfg.RedirectURL == "" {
		return fmt.Errorf("oauth: %s: redirect_url is required", id)
	}
	// ClientSecret is intentionally NOT validated — some deployments use
	// PKCE-only public clients with no secret.
	return nil
}
