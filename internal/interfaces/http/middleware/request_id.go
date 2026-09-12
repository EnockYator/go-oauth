package middleware

import (
	"net/http"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-ID"

// RequestIDMiddleware assigns a unique server-generated identifier to every
// HTTP request.
//
// The identifier is:
//   - returned in the X-Request-ID response header;
//   - stored in the request context
//   - available to logging, error handling, and downstream application code.
//
// Client-provided request IDs are intentionally ignored so that external
// callers cannot control the application's correlation identifiers.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		ctx := r.Context()

		w.Header().Set(requestIDHeader, requestID)

		ctx = requestcontext.WithRequestID(ctx, requestID)

		// Add to span
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("request.id", requestID))

		next.ServeHTTP(
			w,
			r.WithContext(ctx),
		)
	})
}
