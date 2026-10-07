// Package oauth defines the abstraction over OAuth2 providers used by the
// authentication module.
//
// Providers expose only the three operations the application needs:
//   - building the authorization URL,
//   - exchanging the authorization code, and
//   - fetching the normalized user profile.
//
// The concrete HTTP semantics of each
// provider (endpoints, headers, response shapes) are entirely encapsulated.
package oauth

import (
	"context"

	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"golang.org/x/oauth2"
)

// Provider is the port implemented by every OAuth2 provider integration.
type Provider interface {
	// ID returns the stable identifier for this provider.
	ID() auth.ProviderID

	// AuthCodeURL builds the URL the user is redirected to in order to
	// authenticate and authorize.
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string

	// Exchange trades an authorization code for tokens. opts typically
	// includes the PKCE verifier (oauth2.VerifierOption).
	Exchange(
		ctx context.Context,
		code string,
		opts ...oauth2.AuthCodeOption,
	) (*oauth2.Token, error)

	// FetchUserInfo calls the provider's userinfo endpoint using the
	// access token and returns the normalized identity.
	FetchUserInfo(
		ctx context.Context,
		token *oauth2.Token,
	) (*auth.UserInfo, error)
}

// ProviderConfig carries the per-provider configuration needed by an
// integration. It is intentionally transport-agnostic so it can be built
// from environment variables, YAML, or a secrets manager.
type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}
