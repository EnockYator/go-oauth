package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Config defines the configuration for OpenTelemetry tracing.
type Config struct {
	ServiceName     string
	ServiceVersion  string
	DeploymentEnv   string
	OTLPEndpoint    string
	OTLPHeaders     string
	SamplingRatio   float64
	ShutdownTimeout time.Duration
}

// Init initializes OpenTelemetry tracing with the given configuration.
//
// It returns a TracerProvider that should be shut down when the application
// exits. The returned TracerProvider is also set as the global tracer provider.
func Init(
	ctx context.Context,
	cfg Config,
	logger *slog.Logger,
) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("tracing: service name is required")
	}

	if cfg.SamplingRatio < 0 || cfg.SamplingRatio > 1 {
		return nil, fmt.Errorf(
			"tracing: sampling ratio must be between 0 and 1",
		)
	}

	// The OTLP HTTP exporter reads standard OpenTelemetry
	// environment variables such as:
	//
	// OTEL_EXPORTER_OTLP_ENDPOINT
	// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
	// OTEL_EXPORTER_OTLP_HEADERS
	//
	// Therefore we only explicitly configure values that
	// belong to the application itself.
	exporterOptions := []otlptracehttp.Option{}

	if cfg.OTLPEndpoint != "" {
		exporterOptions = append(
			exporterOptions,
			otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		)
	}

	if cfg.OTLPHeaders != "" {
		exporterOptions = append(
			exporterOptions,
			otlptracehttp.WithHeaders(parseHeaders(cfg.OTLPHeaders)),
		)
	}

	exporter, err := otlptracehttp.New(
		ctx,
		exporterOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create OTLP trace exporter: %w",
			err,
		)
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentName(cfg.DeploymentEnv),
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create tracing resource: %w",
			err,
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),

		sdktrace.WithSampler(
			sdktrace.ParentBased(
				sdktrace.TraceIDRatioBased(cfg.SamplingRatio),
			),
		),

		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)

	// W3C Trace Context is the default propagator used by
	// modern OpenTelemetry Go applications, so no custom
	// propagator is necessary here.
	logger.Info(
		"OpenTelemetry tracing initialized",
		"service.name", cfg.ServiceName,
		"service.version", cfg.ServiceVersion,
		"deployment.environment", cfg.DeploymentEnv,
		"sampling_ratio", cfg.SamplingRatio,
	)

	return tp, nil
}

// Shutdown gracefully shuts down the given TracerProvider, ensuring that all
// spans are exported before the application exits.
func Shutdown(
	ctx context.Context,
	tp *sdktrace.TracerProvider,
) error {
	if tp == nil {
		return nil
	}

	return tp.Shutdown(ctx)
}

// parseHeaders parses a comma-separated list of key=value pairs into a map.
func parseHeaders(value string) map[string]string {
	headers := make(map[string]string)

	for _, pair := range splitHeaderPairs(value) {
		key, val, ok := splitHeader(pair)
		if !ok {
			continue
		}

		headers[key] = val
	}

	return headers
}

// splitHeaderPairs splits a comma-separated list of key=value pairs into individual pairs.
func splitHeaderPairs(value string) []string {
	var result []string

	for _, pair := range splitByComma(value) {
		if pair != "" {
			result = append(result, pair)
		}
	}

	return result
}

// splitByComma splits a string by commas, taking care to handle empty segments.
func splitByComma(value string) []string {
	var result []string

	start := 0

	for i := 0; i < len(value); i++ {
		if value[i] == ',' {
			result = append(result, value[start:i])
			start = i + 1
		}
	}

	result = append(result, value[start:])

	return result
}

// splitHeader splits a key=value pair into its key and value components.
// It returns the key, value, and a boolean indicating whether the split was successful.
func splitHeader(value string) (string, string, bool) {
	for i := 0; i < len(value); i++ {
		if value[i] == '=' {
			key := value[:i]
			val := value[i+1:]

			if key == "" {
				return "", "", false
			}

			return key, val, true
		}
	}

	return "", "", false
}
