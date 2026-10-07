package application

import (
	"context"
	"errors"
	auth "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	user "github.com/EnockYator/go-oauth/internal/modules/user/domain"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"time"
)

const touchThreshold = 5 * time.Minute

type ValidateSession struct {
	sessions auth.SessionRepository
	users    UserFinder
	tokens   SessionTokenService
	logger   *slog.Logger
	tracer   trace.Tracer
	now      func() time.Time
}

func NewValidateSession(sessions auth.SessionRepository, users UserFinder, tokens SessionTokenService, logger *slog.Logger, tracer trace.Tracer) *ValidateSession {
	return &ValidateSession{sessions: sessions, users: users, tokens: tokens, logger: logger, tracer: tracer, now: time.Now}
}
func (uc *ValidateSession) Execute(ctx context.Context, rawToken string) (*user.User, error) {
	ctx, span := uc.tracer.Start(ctx, "auth.validate_session")
	defer span.End()
	if rawToken == "" {
		return nil, unauthorized(ctx, "session cookie missing")
	}
	idHash := uc.tokens.Hash(rawToken)
	sess, err := uc.sessions.GetActiveSessionByIDHash(ctx, idHash)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrSessionNotFound), errors.Is(err, auth.ErrSessionExpired), errors.Is(err, auth.ErrSessionRevoked):
			return nil, unauthorized(ctx, "session is not valid")
		default:
			return nil, apperror.New(ctx, apperror.CodeDBQueryFailed, "failed to load session", err)
		}
	}
	u, err := uc.users.GetUserByID(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, unauthorized(ctx, "session is not valid")
		}
		return nil, apperror.New(ctx, apperror.CodeDBQueryFailed, "failed to load user", err)
	}
	if uc.now().Sub(sess.LastSeen) > touchThreshold {
		if err := uc.sessions.TouchSessionLastSeen(ctx, idHash); err != nil {
			uc.logger.WarnContext(ctx, "touch last_seen failed", slog.Any("error", err))
		}
	}
	span.SetAttributes(attribute.String("auth.user_id", u.ID), attribute.String("auth.provider", u.Provider))
	return u, nil
}
func unauthorized(ctx context.Context, msg string) *apperror.AppError {
	return apperror.New(ctx, apperror.CodeAuthSessionExpired, msg, nil)
}
