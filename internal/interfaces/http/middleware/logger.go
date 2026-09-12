package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"

	"go.opentelemetry.io/otel/trace"
)

// LoggerMiddleware records one structured access log entry for each HTTP
// request.
//
// It records:
//   - request ID
//   - trace ID
//   - HTTP method
//   - request path
//   - remote address
//   - user agent
//   - response status
//   - response size
//   - request duration
//
// The middleware does not log sensitive request data such as authorization
// headers, cookies, request bodies, or JWTs.
func LoggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Reuse an existing response recorder when an outer middleware
			// already created one. Otherwise create one here.
			rw, ok := w.(*responseRecorder)
			if !ok {
				rw = &responseRecorder{
					ResponseWriter: w,
				}
			}

			next.ServeHTTP(rw, r)

			traceID := traceIDFromRequest(r)

			requestID := requestcontext.GetRequestID(
				r.Context(),
			)

			slog.InfoContext(
				r.Context(),
				"http request completed",
				slog.String("request_id", requestID),
				slog.String("trace_id", traceID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.Int("status", rw.Status()),
				slog.Int64("bytes", rw.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// traceIDFromRequest returns the trace ID associated with the request's
// active OpenTelemetry span.
func traceIDFromRequest(r *http.Request) string {
	span := trace.SpanContextFromContext(r.Context())

	if !span.IsValid() {
		return ""
	}

	return span.TraceID().String()
}
