package application

import (
	"context"
	"time"

	auth "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	user "github.com/EnockYator/go-oauth/internal/modules/user/domain"
)

type UserUpserter interface {
	UpsertUserByProviderSubject(ctx context.Context, params user.UpsertParams) (*user.User, error)
}

type UserFinder interface {
	GetUserByID(ctx context.Context, id string) (*user.User, error)
}

type OAuthProvider interface {
	AuthCodeURL(state, codeVerifier string) string
	Authenticate(ctx context.Context, code, codeVerifier string) (*auth.UserInfo, error)
}

type OAuthProviderRegistry interface {
	Get(id auth.ProviderID) (OAuthProvider, bool)
}

type StatePayload struct {
	State        string
	CodeVerifier string
	Provider     string
	ReturnTo     string
}

type StateCodec interface {
	Encode(payload StatePayload) (string, error)
	Decode(encoded string) (*StatePayload, error)
	TTL() time.Duration
}

type SessionTokenService interface {
	Generate() (string, error)
	Hash(rawToken string) string
}
