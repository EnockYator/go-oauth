package application

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"time"

	auth "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	user "github.com/EnockYator/go-oauth/internal/modules/user/domain"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type HandleCallbackInput struct {
	Provider    auth.ProviderID
	StateParam  string
	CodeParam   string
	StateCookie string
}

type HandleCallbackOutput struct {
	User         *user.User
	SessionToken string
	SessionTTL   time.Duration
	ReturnTo     string
}

type HandleCallback struct {
	providers  OAuthProviderRegistry
	codec      StateCodec
	sessions   auth.SessionRepository
	users      UserUpserter
	tokens     SessionTokenService
	logger     *slog.Logger
	tracer     trace.Tracer
	sessionTTL time.Duration
	now        func() time.Time
}

func NewHandleCallback(
	providers OAuthProviderRegistry,
	codec StateCodec,
	sessions auth.SessionRepository,
	users UserUpserter,
	tokens SessionTokenService,
	logger *slog.Logger,
	tracer trace.Tracer,
	sessionTTL time.Duration,
) *HandleCallback {
	return &HandleCallback{providers: providers, codec: codec, sessions: sessions, users: users, tokens: tokens, logger: logger, tracer: tracer, sessionTTL: sessionTTL, now: time.Now}
}

func (uc *HandleCallback) Execute(ctx context.Context, in HandleCallbackInput) (*HandleCallbackOutput, error) {
	ctx, span := uc.tracer.Start(ctx, "auth.handle_callback", trace.WithAttributes(attribute.String("oauth.provider", string(in.Provider))))
	defer span.End()

	payload, err := uc.codec.Decode(in.StateCookie)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "state decode failed")
		return nil, apperror.New(ctx, apperror.CodeUnauthorized, "invalid oauth state", auth.ErrStateInvalid)
	}
	if subtle.ConstantTimeCompare([]byte(payload.State), []byte(in.StateParam)) != 1 {
		span.RecordError(auth.ErrStateMismatch)
		span.SetStatus(codes.Error, "state mismatch")
		return nil, apperror.New(ctx, apperror.CodeUnauthorized, "invalid oauth state", auth.ErrStateMismatch)
	}
	if payload.Provider != string(in.Provider) {
		return nil, apperror.New(ctx, apperror.CodeUnauthorized, "provider mismatch", auth.ErrStateMismatch)
	}

	provider, ok := uc.providers.Get(in.Provider)
	if !ok {
		return nil, apperror.New(ctx, apperror.CodeBadRequest, "unknown provider", auth.ErrProviderUnknown)
	}

	info, err := provider.Authenticate(ctx, in.CodeParam, payload.CodeVerifier)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "oauth authentication failed")
		return nil, apperror.New(ctx, apperror.CodeExternalServiceFailure, "failed to authenticate with provider", auth.ErrCodeExchangeFailed)
	}
	if err := info.Validate(); err != nil {
		return nil, apperror.New(ctx, apperror.CodeExternalServiceFailure, "provider returned invalid user profile", err)
	}

	u, err := uc.users.UpsertUserByProviderSubject(ctx, user.UpsertParams{Email: info.Email, Name: info.Name, AvatarURL: stringPtr(info.AvatarURL), Provider: string(info.Provider), ProviderSubject: info.ProviderSubject})
	if err != nil {
		span.RecordError(err)
		return nil, apperror.New(ctx, apperror.CodeDBQueryFailed, "failed to persist user", err)
	}

	rawToken, err := uc.tokens.Generate()
	if err != nil {
		return nil, apperror.New(ctx, apperror.CodeInternalServerError, "failed to create session", err)
	}

	metadata, _ := json.Marshal(map[string]any{"provider": string(info.Provider)})
	now := uc.now()
	sess := &auth.Session{IDHash: uc.tokens.Hash(rawToken), UserID: u.ID, ExpiresAt: now.Add(uc.sessionTTL), Metadata: metadata}
	if err := uc.sessions.CreateSession(ctx, sess); err != nil {
		span.RecordError(err)
		return nil, apperror.New(ctx, apperror.CodeDBQueryFailed, "failed to persist session", err)
	}

	span.SetAttributes(attribute.String("auth.user_id", u.ID))
	uc.logger.InfoContext(ctx, "oauth login completed", slog.String("provider", string(info.Provider)), slog.String("user_id", u.ID))

	return &HandleCallbackOutput{User: u, SessionToken: rawToken, SessionTTL: uc.sessionTTL, ReturnTo: sanitizeReturnTo(payload.ReturnTo)}, nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
