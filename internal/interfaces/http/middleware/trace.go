package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// type TraceMiddlewareOption configures the TraceMiddleware.
type TraceMiddlewareOption func(*TraceMiddlewareConfig)

// TraceMiddlewareConfig holds configuration for the TraceMiddleware.
// - ServiceName: The name of the service, used in span attributes and the tracer name.
// - tracerProvider: The OpenTelemetry tracer provider to use for creating spans.
// - propagators: The OpenTelemetry propagators to use for extracting and injecting trace context.
// - spanNameFormatter: A function that formats the span name based on the HTTP method and request.
// - filter: A function that determines whether to skip tracing for a given request.
// - publicEndpoint: A boolean indicating whether the endpoint is public (no authentication required).
type TraceMiddlewareConfig struct {
	serviceName       string
	tracerProvider    trace.TracerProvider
	propagators       propagation.TextMapPropagator
	spanNameFormatter func(string, *http.Request) string
	filter            func(*http.Request) bool
	publicEndpoint    bool
}

// WithServiceName sets the service name for the TraceMiddleware.
func WithServiceName(name string) TraceMiddlewareOption {
	return func(cfg *TraceMiddlewareConfig) {
		cfg.serviceName = name
	}
}

// WithTracerProvider sets a custom TracerProvider. Defaults to otel.GetTracerProvider().
func WithTracerProvider(tp trace.TracerProvider) TraceMiddlewareOption {
	return func(cfg *TraceMiddlewareConfig) {
		cfg.tracerProvider = tp
	}
}

// WithPropagators sets custom propagators for the TraceMiddleware. Defaults to otel.GetTextMapPropagator().
func WithPropagators(propagators propagation.TextMapPropagator) TraceMiddlewareOption {
	return func(cfg *TraceMiddlewareConfig) {
		cfg.propagators = propagators
	}
}

// WithSpanNameFormatter customizes how the span is named.
// Default: "METHOD /path".
func WithSpanNameFormatter(formatter func(method string, r *http.Request) string) TraceMiddlewareOption {
	return func(cfg *TraceMiddlewareConfig) {
		cfg.spanNameFormatter = formatter
	}
}

// WithFilter skips tracing for requests where the filter returns true.
// Health checks (/health, etc.), metrics (/metrics, /debug/pprof) endpoints, or static assets can be excluded this way.
//
// Note: returning true means "exclude from tracing".
func WithFilter(filter func(r *http.Request) bool) TraceMiddlewareOption {
	return func(cfg *TraceMiddlewareConfig) {
		cfg.filter = filter
	}
}

// WithPublicEndpoint marks the endpoint as public, meaning no authentication is required.
func WithPublicEndpointFn() TraceMiddlewareOption {
	return func(cfg *TraceMiddlewareConfig) {
		cfg.publicEndpoint = true
	}
}

// NewTraceMiddleware returns a middleware that starts and ends a server span for each request.
//
// The middleware should be the outermost layer so that all other middlewares
// (request ID, auth, etc.) run inside the span.
//
// Usage:
//
//	handler := middlware.NewTraceMiddleware(
//	    middleware.WithServiceName("saas-photo-listing-platform"),
//	    middleware.WithFilter(func(r *http.Request) bool {
//	        return r.URL.Path == "/health" || r.URL.Path == "/metrics"
//	    }),
//	    middleware.WithPublicEndpoint(), // treat all incoming requests as public endpoints (no auth required)
//	)(myHandler)
func NewTraceMiddleware(opts ...TraceMiddlewareOption) func(http.Handler) http.Handler {
	cfg := &TraceMiddlewareConfig{
		serviceName:    "saas-photo-listing-platform", // default service name
		tracerProvider: otel.GetTracerProvider(),
		propagators:    otel.GetTextMapPropagator(),
		spanNameFormatter: func(method string, r *http.Request) string {
			return method + " " + r.URL.Path
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build otelhttp options
	otelOpts := []otelhttp.Option{
		otelhttp.WithTracerProvider(cfg.tracerProvider),
		otelhttp.WithPropagators(cfg.propagators),
		otelhttp.WithSpanNameFormatter(cfg.spanNameFormatter),
	}
	if cfg.filter != nil {
		otelOpts = append(otelOpts, otelhttp.WithFilter(func(r *http.Request) bool {
			return cfg.filter(r) // otelhttp's filter returns true to exclude from tracing
		}))
	}
	if cfg.publicEndpoint {
		otelOpts = append(otelOpts, otelhttp.WithPublicEndpointFn(func(r *http.Request) bool {
			return true
		}))
	}

	// Create the otelhttp middleware
	baseMiddleware := otelhttp.NewMiddleware(cfg.serviceName, otelOpts...)

	// Return a wrapper that adds a constant "service.name" attribute to every span.
	// This is optional but useful if you want the attribute even when the span name
	// is overridden by a custom formatter.
	return func(next http.Handler) http.Handler {
		return baseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(attribute.String("service.name", cfg.serviceName))
			next.ServeHTTP(w, r)
		}))
	}
}
