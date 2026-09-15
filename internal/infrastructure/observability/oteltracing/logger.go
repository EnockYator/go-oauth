package oteltracing

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// LoggerOption customizes the default logger construction.
type LoggerOption func(*slog.HandlerOptions)

// WithLevel overrides the default log level.
func WithLevel(level slog.Level) LoggerOption {
	return func(o *slog.HandlerOptions) { o.Level = level }
}

// WithAddSource toggles source-file attribution on every log record.
func WithAddSource(add bool) LoggerOption {
	return func(o *slog.HandlerOptions) { o.AddSource = add }
}

// NewLogger returns a slog.Logger decorated with trace correlation.
//
// In production it emits JSON; otherwise it emits human-readable text.
// Every call to logger.InfoContext/ErrorContext/... with a context that
// carries a valid span will automatically include "trace_id" and
// "span_id" attributes, enabling log-to-trace navigation in Grafana,
// Datadog, Loki, Jaeger, etc.
//
//	ctx, span := tracer.Start(ctx, "handle")
//	defer span.End()
//	logger.InfoContext(ctx, "processing") // auto-correlated
func NewLogger(
	env string,
	opts ...LoggerOption,
) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	for _, opt := range opts {
		opt(handlerOpts)
	}

	var base slog.Handler
	if env == "production" {
		base = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		handlerOpts.AddSource = true
		base = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	return slog.New(traceHandler{base})
}

// ---------------------------------------------------------------------
// traceHandler
// ---------------------------------------------------------------------

// traceHandler decorates any slog.Handler with OpenTelemetry trace
// correlation. It must be used together with ctx-aware slog calls:
// logger.InfoContext(ctx, ...), logger.ErrorContext(ctx, ...), etc.
type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return traceHandler{h.Handler.WithGroup(name)}
}