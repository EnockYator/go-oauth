package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// NewTimeout creates a middleware that applies a maximum request-processing
// deadline.
//
// The timeout is enforced through context cancellation. It does not forcibly
// terminate a handler. Downstream operations must respect the request context.
func NewTimeout(timeout time.Duration) (func(http.Handler) http.Handler, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf(
			"invalid timeout configuration: timeout must be greater than zero",
		)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If an upstream component already established a shorter
			// deadline, preserve it exactly as provided.
			if deadline, ok := r.Context().Deadline(); ok {
				if time.Until(deadline) <= timeout {
					next.ServeHTTP(w, r)
					return
				}
			}

			ctx, cancel := context.WithTimeout(
				r.Context(),
				timeout,
			)

			defer cancel()

			// Reuse the shared response recorder when one already exists.
			//
			// This allows the middleware to determine whether it is still
			// safe to write the timeout response without creating another
			// ResponseWriter wrapper.
			rw, ok := w.(*responseRecorder)
			if !ok {
				rw = &responseRecorder{
					ResponseWriter: w,
				}
			}

			span := trace.SpanFromContext(ctx)

			span.SetAttributes(
				attribute.Int64("http.request.timeout_ms", timeout.Milliseconds()),
			)

			next.ServeHTTP(rw, r.WithContext(ctx))

			// If the context expired because of this middleware's deadline,
			// attempt to return the standard timeout response.
			//
			// If the handler already committed a response, it is too late
			// to replace that response safely.
			if ctx.Err() != context.DeadlineExceeded {
				return
			}

			if rw.Committed() {
				return
			}

			span.SetStatus(codes.Error, "request deadline exceeded")
			span.SetAttributes(
				attribute.Bool("http.request.timeout", true),
			)

			response.WriteError(
				rw,
				r,
				apperror.New(
					r.Context(),
					apperror.CodeRequestTimeout,
					"request timed out",
					nil,
				),
			)
		})
	}, nil
}
