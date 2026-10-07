package application

import (
	"context"
	auth "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

type Logout struct {
	sessions auth.SessionRepository
	tokens   SessionTokenService
	logger   *slog.Logger
	tracer   trace.Tracer
}

func NewLogout(sessions auth.SessionRepository, tokens SessionTokenService, logger *slog.Logger, tracer trace.Tracer) *Logout {
	return &Logout{sessions: sessions, tokens: tokens, logger: logger, tracer: tracer}
}
func (uc *Logout) Execute(ctx context.Context, rawToken string) error {
	ctx, span := uc.tracer.Start(ctx, "auth.logout")
	defer span.End()
	if rawToken == "" {
		return nil
	}
	if err := uc.sessions.RevokeSessionByIDHash(ctx, uc.tokens.Hash(rawToken)); err != nil {
		return apperror.New(ctx, apperror.CodeDBQueryFailed, "failed to revoke session", err)
	}
	uc.logger.InfoContext(ctx, "session revoked")
	return nil
}
