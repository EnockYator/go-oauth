package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/cookie"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	user "github.com/EnockYator/go-oauth/internal/modules/user/domain"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type authContextKey struct{}

var userKey = authContextKey{}

// SessionValidator is the middleware's port onto the auth application
// layer. It is satisfied by *application.ValidateSession.
type SessionValidator interface {
	Execute(ctx context.Context, rawToken string) (*user.User, error)
}

// AuthMiddleware authenticates the request by validating the session
// cookie. On success it:
//   - stores the authenticated user in the request context (retrievable
//     via GetUser);
//   - populates requestcontext with the user ID so that apperror.New can
//     attach it to error records automatically;
//   - annotates the active span with the user and provider.
//
// Mount only on routes that require authentication. Public routes must
// either omit this middleware or be registered on a separate mux.
func AuthMiddleware(
	validator SessionValidator,
	cookieCfg cookie.SessionConfig,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			span := trace.SpanFromContext(ctx)

			rawToken, ok := cookie.ReadSession(r, cookieCfg)
			if !ok {
				span.SetAttributes(attribute.String("auth.status", "no_cookie"))
				writeUnauthorized(w, r, "authentication required")
				return
			}

			u, err := validator.Execute(ctx, rawToken)
			if err != nil {
				// ValidateSession always returns *apperror.AppError here,
				// but defensive handling keeps a buggy implementation from
				// leaking a nil dereference.
				var appErr *apperror.AppError
				if !errors.As(err, &appErr) {
					appErr = apperror.New(
						ctx,
						apperror.CodeInternalServerError,
						"authentication failed",
						err,
					)
				}

				span.SetStatus(codes.Error, "session invalid")
				span.SetAttributes(
					attribute.String("auth.status", "invalid"),
					attribute.String("auth.error_code", string(appErr.Code)),
				)
				response.WriteError(w, r, appErr)
				return
			}

			ctx = context.WithValue(ctx, userKey, u)
			ctx = requestcontext.WithUserID(ctx, u.ID)

			span.SetAttributes(
				attribute.String("auth.status", "ok"),
				attribute.String("auth.user_id", u.ID),
				attribute.String("auth.provider", u.Provider),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser returns the authenticated user stored by AuthMiddleware.
func GetUser(ctx context.Context) (*user.User, bool) {
	u, ok := ctx.Value(userKey).(*user.User)
	return u, ok
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request, msg string) {
	response.WriteError(w, r, apperror.New(
		r.Context(),
		apperror.CodeAuthSessionExpired,
		msg,
		nil,
	))
}
