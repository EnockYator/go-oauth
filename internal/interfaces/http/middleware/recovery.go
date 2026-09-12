package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RecoveryMiddleware recovers from panics that occur during HTTP request
// processing.
//
// It:
//   - prevents request panics from escaping the HTTP middleware chain;
//   - records the panic on the active OpenTelemetry span;
//   - logs the panic and stack trace;
//   - prevents panic details from being exposed to clients;
//   - writes the standard internal-server-error response when the response
//     has not already been committed.
//
// Recovery should be placed outside the middleware and handler components
// whose panics it is expected to recover.
func RecoveryMiddleware() func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reuse the shared response recorder when a middleware farther
			// out in the chain has already created one.
			rw, ok := w.(*responseRecorder)
			if !ok {
				rw = &responseRecorder{
					ResponseWriter: w,
				}
			}

			defer func() {
				if rec := recover(); rec != nil {
					handleRecoveredPanic(
						rw,
						r,
						rec,
					)
				}
			}()

			next.ServeHTTP(rw, r)
		})
	}
}

// handleRecoveredPanic records, logs, and safely responds to a recovered
// panic.
//
// The original panic value is intentionally never returned to the client.
func handleRecoveredPanic(
	rw *responseRecorder,
	r *http.Request,
	rec any,
) {
	stack := debug.Stack()

	panicErr := fmt.Errorf(
		"panic recovered: %v",
		rec,
	)

	recordPanicSpan(
		r,
		panicErr,
		rec,
		stack,
	)

	logPanic(
		r,
		rec,
		stack,
	)

	// Once HTTP response headers/body have been committed, replacing the
	// response is no longer safe or possible.
	if rw.Committed() {
		return
	}

	response.WriteError(
		rw,
		r,
		apperror.New(
			r.Context(),
			apperror.CodeInternalServerError,
			"internal server error",
			nil,
		),
	)
}

// recordPanicSpan records panic information on the active OpenTelemetry span.
func recordPanicSpan(
	r *http.Request,
	panicErr error,
	rec any,
	stack []byte,
) {
	span := trace.SpanFromContext(r.Context())

	if !span.SpanContext().IsValid() {
		return
	}

	span.RecordError(
		panicErr,
		trace.WithAttributes(
			attribute.String("exception.type", fmt.Sprintf("%T", rec)),
			attribute.String("exception.message", fmt.Sprint(rec)),
			attribute.String("exception.stacktrace", string(stack)),
		),
	)

	span.SetStatus(codes.Error, "panic recovered")

	span.SetAttributes(
		attribute.Bool("http.panic.recovered", true),
	)
}

// logPanic writes the full panic diagnostics to the structured logger.
//
// Sensitive implementation details are intentionally kept out of the HTTP
// response and are recorded only in internal observability systems.
func logPanic(
	r *http.Request,
	rec any,
	stack []byte,
) {
	slog.ErrorContext(
		r.Context(),
		"panic recovered from HTTP request",
		slog.Any("panic", rec),
		slog.String("stack_trace", string(stack)),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)
}
