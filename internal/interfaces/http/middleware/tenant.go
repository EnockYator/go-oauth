package middleware

import (
	"net/http"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TenantMiddleware enforces the presence of an authenticated tenant identity
// in the request context.
//
// The tenant ID must have been established by AuthMiddleware from validated
// authentication claims. This middleware never accepts tenant identity from
// client-controlled HTTP input and never replaces the authenticated tenant
// value.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := requestcontext.GetTenantID(r.Context())
		span := trace.SpanFromContext(r.Context())

		if tenantID == "" {
			response.WriteError(
				w,
				r,
				apperror.New(
					r.Context(),
					apperror.CodeTenantIDMissing,
					"tenant context required",
					nil,
				),
			)

			return
		}

		if span.SpanContext().IsValid() {
			span.SetAttributes(attribute.String("tenant.id", tenantID))
		}

		next.ServeHTTP(w, r)
	})
}
