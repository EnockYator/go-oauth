package oauth

import (
	"context"
	authapp "github.com/EnockYator/go-oauth/internal/modules/auth/application"
	authdomain "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"golang.org/x/oauth2"
)

type ApplicationRegistry struct{ registry *Registry }

func NewApplicationRegistry(registry *Registry) *ApplicationRegistry {
	return &ApplicationRegistry{registry: registry}
}
func (r *ApplicationRegistry) Get(id authdomain.ProviderID) (authapp.OAuthProvider, bool) {
	p, ok := r.registry.Get(id)
	if !ok {
		return nil, false
	}
	return applicationProvider{provider: p}, true
}

type applicationProvider struct{ provider Provider }

func (p applicationProvider) AuthCodeURL(state, verifier string) string {
	return p.provider.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}
func (p applicationProvider) Authenticate(ctx context.Context, code, verifier string) (*authdomain.UserInfo, error) {
	token, err := p.provider.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, err
	}
	return p.provider.FetchUserInfo(ctx, token)
}
