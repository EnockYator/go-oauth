package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	auth "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2"
)

type InitiateLoginInput struct {
	Provider auth.ProviderID
	ReturnTo string
}
type InitiateLoginOutput struct {
	RedirectURL string
	StateCookie string
	StateTTL    time.Duration
}

type InitiateLogin struct {
	providers OAuthProviderRegistry
	codec     StateCodec
	logger    *slog.Logger
	tracer    trace.Tracer
}

func NewInitiateLogin(providers OAuthProviderRegistry, codec StateCodec, logger *slog.Logger, tracer trace.Tracer) *InitiateLogin {
	return &InitiateLogin{providers: providers, codec: codec, logger: logger, tracer: tracer}
}

func (uc *InitiateLogin) Execute(ctx context.Context, in InitiateLoginInput) (*InitiateLoginOutput, error) {
	ctx, span := uc.tracer.Start(ctx, "auth.initiate_login", trace.WithAttributes(attribute.String("oauth.provider", string(in.Provider))))
	defer span.End()
	provider, ok := uc.providers.Get(in.Provider)
	if !ok {
		span.RecordError(auth.ErrProviderUnknown)
		return nil, apperror.New(ctx, apperror.CodeBadRequest, "unknown provider", auth.ErrProviderUnknown)
	}
	returnTo := sanitizeReturnTo(in.ReturnTo)
	state, err := generateState()
	if err != nil {
		return nil, apperror.New(ctx, apperror.CodeInternalServerError, "failed to start login", err)
	}
	verifier := oauth2.GenerateVerifier()
	encoded, err := uc.codec.Encode(StatePayload{State: state, CodeVerifier: verifier, Provider: string(in.Provider), ReturnTo: returnTo})
	if err != nil {
		return nil, apperror.New(ctx, apperror.CodeInternalServerError, "failed to start login", err)
	}
	redirectURL := provider.AuthCodeURL(state, verifier)
	uc.logger.InfoContext(ctx, "oauth login initiated", slog.String("provider", string(in.Provider)), slog.String("return_to", returnTo))
	return &InitiateLoginOutput{RedirectURL: redirectURL, StateCookie: encoded, StateTTL: uc.codec.TTL()}, nil
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sanitizeReturnTo(in string) string {
	in = strings.TrimSpace(in)
	if in == "" || in[0] != '/' || strings.HasPrefix(in, "//") || strings.ContainsAny(in, "\r\n") {
		return "/"
	}
	return in
}
