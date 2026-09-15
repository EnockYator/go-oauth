package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// LoggerMiddleware records one structured access log entry per HTTP request
// and enriches the request's active OpenTelemetry span with response data.
//
// Ordering contract — this middleware MUST be installed *inside* an
// OpenTelemetry HTTP instrumentation middleware (e.g. otelhttp.NewMiddleware)
// so that r.Context() already carries the server span. A typical chain is:
//
//	otelhttp.NewMiddleware("http.server")(      // starts the server span
//	    requestIDMiddleware(                    // adds request id to ctx
//	        LoggerMiddleware(logger)(           // <- this one
//	            router,
//	        ),
//	    ),
//	)
//
// When a span is present:
//   - trace_id and span_id are injected automatically by the tracing
//     package's traceHandler — the middleware does NOT extract them;
//   - the log level is derived from the HTTP status
//     (2xx/3xx → Info, 4xx → Warn, 5xx → Error);
//   - the span is annotated with status, body size, route, and request id.
//
// Sensitive data (Authorization, Cookie, bodies, JWTs) is never logged.
func LoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw, ok := w.(*responseRecorder)
			if !ok {
				rw = &responseRecorder{ResponseWriter: w}
			}

			ctx := r.Context()
			span := trace.SpanFromContext(ctx)
			requestID := requestcontext.GetRequestID(ctx)

			// Annotate the span *before* the handler runs so that any
			// long-running request shows the request id while it is
			// still in flight.
			if span.IsRecording() && requestID != "" {
				span.SetAttributes(
					attribute.String("http.request_id", requestID),
				)
			}

			next.ServeHTTP(rw, r)

			status := rw.Status()
			bytesWritten := rw.BytesWritten()
			duration := time.Since(start)

			// r.Pattern is populated by ServeMux during routing, i.e.
			// inside next.ServeHTTP, so it must be read AFTER the call.
			route := routePattern(r)

			// ---- Enrich the span ------------------------------------
			if span.IsRecording() {
				span.SetAttributes(
					attribute.String("http.route", route),
					attribute.Int("http.response.status_code", status),
					attribute.Int64("http.response.body.size", bytesWritten),
				)

				if status >= http.StatusInternalServerError {
					span.SetStatus(
						codes.Error,
						http.StatusText(status),
					)
				}
			}

			// ---- Emit the access log --------------------------------
			// Use LogAttrs to avoid the []any allocation on the hot
			// path. trace_id and span_id are appended by the
			// tracing.traceHandler automatically.
			logger.LogAttrs(
				ctx,
				levelForStatus(status),
				"http request completed",
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("route", route),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.String("proto", r.Proto),
				slog.Int("status", status),
				slog.Int64("bytes", bytesWritten),
				slog.Duration("duration", duration),
			)
		})
	}
}

// levelForStatus maps an HTTP status code to an appropriate log level.
//
// 2xx/3xx → Info
// 4xx     → Warn   (client error; not a server incident, but worth flagging)
// 5xx     → Error  (server incident; should page / alert)
func levelForStatus(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// routePattern returns the low-cardinality route pattern registered with
// the router (Go 1.22+ net/http.ServeMux sets r.Pattern during routing).
// Falls back to the raw URL path when no pattern is available, e.g. when
// the request was handled by a catch-all handler.
func routePattern(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.URL.Path
}