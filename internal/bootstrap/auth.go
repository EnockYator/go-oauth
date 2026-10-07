package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/cookie"
	authapp "github.com/EnockYator/go-oauth/internal/modules/auth/application"
	authrepo "github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/database/repository"
	authsqlc "github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/database/repository/sqlc"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauth"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauthstate"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/session"
	userrepo "github.com/EnockYator/go-oauth/internal/modules/user/infrastructure/database/repository"
	usersqlc "github.com/EnockYator/go-oauth/internal/modules/user/infrastructure/database/repository/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
)

type AuthDependencies struct {
	InitiateLogin   *authapp.InitiateLogin
	HandleCallback  *authapp.HandleCallback
	Logout          *authapp.Logout
	ValidateSession *authapp.ValidateSession
	SessionCookie   cookie.SessionConfig
	StateCookie     cookie.StateConfig
}

func buildAuthDependencies(db *pgxpool.Pool, cfg *config.Config, logger *slog.Logger, tracer trace.Tracer) (*AuthDependencies, error) {
	if db == nil || cfg == nil || logger == nil {
		return nil, fmt.Errorf("auth bootstrap: nil dependency")
	}

	sessions := authrepo.NewSessionRepo(authsqlc.New(db))
	users := userrepo.NewUserRepo(usersqlc.New(db))

	providers, err := oauth.Build(oauth.BuildConfig{Google: toProviderConfig(cfg.Oauth.Google), GitHub: toProviderConfig(cfg.Oauth.GitHub)})
	if err != nil {
		return nil, fmt.Errorf("build oauth providers: %w", err)
	}

	stateCodec, err := oauthstate.NewCodec(cfg.Oauth.StateSecret, cfg.Oauth.StateTTL)
	if err != nil {
		return nil, fmt.Errorf("build oauth state codec: %w", err)
	}

	providerPort := oauth.NewApplicationRegistry(providers)
	statePort := oauthstate.NewApplicationCodec(stateCodec)
	tokenService := session.NewTokenService()

	return &AuthDependencies{
		InitiateLogin:   authapp.NewInitiateLogin(providerPort, statePort, logger, tracer),
		HandleCallback:  authapp.NewHandleCallback(providerPort, statePort, sessions, users, tokenService, logger, tracer, cfg.Oauth.SessionTTL),
		Logout:          authapp.NewLogout(sessions, tokenService, logger, tracer),
		ValidateSession: authapp.NewValidateSession(sessions, users, tokenService, logger, tracer),
		SessionCookie:   cookie.SessionConfig{Secure: cfg.Oauth.SessionCookieSecure, TTL: cfg.Oauth.SessionTTL, SameSite: http.SameSiteLaxMode},
		StateCookie:     cookie.StateConfig{Secure: cfg.Oauth.SessionCookieSecure, TTL: cfg.Oauth.StateTTL},
	}, nil
}

func toProviderConfig(in *config.OAuthProviderConfig) *oauth.ProviderConfig {
	if in == nil {
		return nil
	}
	return &oauth.ProviderConfig{ClientID: in.ClientID, ClientSecret: in.ClientSecret, RedirectURL: in.RedirectURL, Scopes: in.Scopes}
}
