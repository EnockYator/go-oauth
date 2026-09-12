package middleware

import (
	"context"
	"net/http"
	"strings"

	auth "github.com/EnockYator/go-oauth/internal/domain/auth/domain"
	"github.com/EnockYator/go-oauth/internal/domain/auth/infrastructure/jwt"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type authContextKey struct{}

var claimsKey = authContextKey{}

// AuthMiddleware extracts and validates a bearer token, storing the
// resulting Claims in the request context and also populating the
// requestcontext values (user_id, tenant_id, roles).
//
// Should only be mounted on routes that require authentication.
func AuthMiddleware(validator jwt.TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			span := trace.SpanFromContext(ctx)

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.WriteError(
					w,
					r,
					apperror.New(
						ctx,
						apperror.CodeAuthTokenMissing,
						"missing authorization header",
						nil,
					),
				)
				span.SetAttributes(attribute.String("auth.status", "missing_token"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				response.WriteError(
					w,
					r,
					apperror.New(
						ctx,
						apperror.CodeAuthTokenInvalid,
						"invalid authorization format",
						nil,
					),
				)
				span.SetAttributes(attribute.String("auth.status", "invalid_token_format"))
				return
			}

			token := parts[1]

			// Validate token using the provided validator
			claims, err := validator.Validate(token)
			if err != nil || claims == nil {
				span.SetAttributes(attribute.String("auth.status", "invalid_token"))

				code := apperror.CodeAuthTokenInvalid

				if apperror.IsCode(err, apperror.CodeAuthTokenExpired) {
					code = apperror.CodeAuthTokenExpired
				}

				response.WriteError(
					w,
					r,
					apperror.New(
						ctx,
						code,
						"invalid or expired token",
						nil,
					),
				)
				span.SetAttributes(
					attribute.String("auth.status", "invalid_or_expired_token"))
				return
			}

			// Store claims in context (use value copy to prevent mutation)
			ctx = context.WithValue(ctx, claimsKey, claims)

			// Set requestcontext values for easy access
			ctx = requestcontext.WithUserID(ctx, claims.UserID)
			ctx = requestcontext.WithTenantID(ctx, claims.TenantID)
			ctx = requestcontext.WithRoles(ctx, claims.Roles)

			// Add trace attributes for authenticated user
			span.SetAttributes(
				attribute.String("auth.user_id", claims.UserID),
				attribute.String("auth.tenant_id", claims.TenantID),
				attribute.StringSlice("auth.roles", claims.Roles),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims retrieves the authenticated Claims from context.
// Returns the Claims and a boolean indicating if they were found.
func GetClaims(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(auth.Claims)
	return c, ok
}
