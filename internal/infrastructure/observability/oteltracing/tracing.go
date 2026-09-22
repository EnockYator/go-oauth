// Package oteltracing provides a production-grade OpenTelemetry tracing setup.
//
// Design goals:
//
//   - Rich, automatic resource detection (env, process, OS, container).
//   - Both OTLP/gRPC and OTLP/HTTP transports, with TLS.
//   - SDK internal errors routed through the application's slog logger.
//   - W3C Trace Context + Baggage propagation wired explicitly.
//   - Clean, deterministic shutdown.
package oteltracing

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc/credentials"
)

// Init configures the global TracerProvider, propagator, and OTel error
// handler, and returns the provider for lifecycle management by the caller.
//
// The returned TracerProvider is also installed as the global provider, so
// instrumented libraries (otelhttp, otelgrpc, ...) work without further
// wiring.
func Init(
	ctx context.Context,
	cfg Config,
	logger *slog.Logger,
) (*sdktrace.TracerProvider, error) {
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	// -----------------------------------------------------------------
	// 1. Route SDK-internal failures into the app logger.
	//
	//    Without this, export failures vanish silently and the only
	//    symptom is missing traces in the backend.
	// -----------------------------------------------------------------
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error(
			"opentelemetry sdk error",
			slog.Any("error", err),
		)
	}))

	// -----------------------------------------------------------------
	// 2. Build the OTLP exporter.
	// -----------------------------------------------------------------
	exporter, err := newExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	// -----------------------------------------------------------------
	// 3. Build the resource.
	//
	//    Detection is best-effort: a partial failure logs a warning but
	//    must not prevent tracing from being enabled.
	// -----------------------------------------------------------------
	res, err := newResource(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("create tracing resource: %w", err)
	}

	// -----------------------------------------------------------------
	// 4. Assemble the TracerProvider.
	// -----------------------------------------------------------------
	bspOpts := make([]sdktrace.BatchSpanProcessorOption, 0, 3)
	if cfg.BatchTimeout > 0 {
		bspOpts = append(bspOpts, sdktrace.WithBatchTimeout(cfg.BatchTimeout))
	}
	if cfg.MaxExportBatchSize > 0 {
		bspOpts = append(
			bspOpts,
			sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatchSize),
		)
	}
	if cfg.MaxQueueSize > 0 {
		bspOpts = append(bspOpts, sdktrace.WithMaxQueueSize(cfg.MaxQueueSize))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(
			sdktrace.ParentBased(
				sdktrace.TraceIDRatioBased(cfg.SamplingRatio),
			),
		),
		sdktrace.WithBatcher(exporter, bspOpts...),
	)

	// -----------------------------------------------------------------
	// 5. Install globals. Set the propagator explicitly rather than
	//    relying on the SDK default, so behavior is stable across
	//    OpenTelemetry-Go upgrades.
	// -----------------------------------------------------------------
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.InfoContext(ctx,
		"opentelemetry tracing initialized",
		slog.String("App.name", cfg.AppName),
		slog.String("App.version", cfg.AppVersion),
		slog.String("deployment.environment", cfg.DeploymentEnv),
		slog.String("otlp.protocol", string(cfg.Protocol)),
		slog.String("otlp.endpoint", cfg.Endpoint),
		slog.Bool("otlp.insecure", cfg.Insecure),
		slog.Float64("sampling_ratio", cfg.SamplingRatio),
	)

	return tp, nil
}

// Shutdown flushes pending spans and releases the exporter. The caller
// owns the shutdown timeout via ctx.
func Shutdown(
	ctx context.Context,
	tp *sdktrace.TracerProvider,
) error {
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
}

// ---------------------------------------------------------------------
// Exporters
// ---------------------------------------------------------------------

func newExporter(ctx context.Context, cfg Config) (*otlptrace.Exporter, error) {
	switch cfg.Protocol {
	case ProtocolGRPC:
		return newGRPCExporter(ctx, cfg)
	case ProtocolHTTP:
		return newHTTPExporter(ctx, cfg)
	default:
		// Unreachable: normalize() already validated.
		return nil, fmt.Errorf("unsupported protocol %q", cfg.Protocol)
	}
}

func newGRPCExporter(
	ctx context.Context,
	cfg Config,
) (*otlptrace.Exporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithTimeout(cfg.ExportTimeout),
	}

	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}
	if cfg.Compression == "gzip" {
		opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
	}

	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		tlsCfg, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			opts = append(
				opts,
				otlptracegrpc.WithTLSCredentials(
					credentials.NewTLS(tlsCfg),
				),
			)
		}
	}

	return otlptracegrpc.New(ctx, opts...)
}

func newHTTPExporter(
	ctx context.Context,
	cfg Config,
) (*otlptrace.Exporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithTimeout(cfg.ExportTimeout),
	}

	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	if cfg.Compression == "gzip" {
		opts = append(
			opts,
			otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
		)
	}

	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	} else {
		tlsCfg, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			opts = append(opts, otlptracehttp.WithTLSClientConfig(tlsCfg))
		}
	}

	return otlptracehttp.New(ctx, opts...)
}

// buildTLSConfig returns nil when no custom TLS material was provided,
// signalling the exporter to use its own defaults.
func buildTLSConfig(cfg Config) (*tls.Config, error) {
	if cfg.TLSCAFile == "" && cfg.TLSCertFile == "" &&
		cfg.TLSKeyFile == "" && cfg.TLSServerName == "" {
		return nil, nil
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.TLSServerName != "" {
		tlsCfg.ServerName = cfg.TLSServerName
	}

	if cfg.TLSCAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf(
				"TLS CA file %q contains no valid certificates",
				cfg.TLSCAFile,
			)
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			return nil, fmt.Errorf(
				"both TLSCertFile and TLSKeyFile must be set for mTLS",
			)
		}
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// ---------------------------------------------------------------------
// Resource
// ---------------------------------------------------------------------

// newResource builds the resource with rich, automatic detection.
//
// If resource.New returns a partial resource plus an error (e.g. the
// container detector failed because cgroups are unavailable), we log a
// warning and continue - losing a single attribute must not disable
// tracing.
func newResource(
	ctx context.Context,
	cfg Config,
	logger *slog.Logger,
) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),      // OTEL_RESOURCE_ATTRIBUTES, OTEL_App_NAME
		resource.WithTelemetrySDK(), // telemetry.sdk.{name,version,language}
		resource.WithProcess(),      // process.{pid,executable,...}
		resource.WithOS(),           // os.{type,description}
		resource.WithContainer(),    // container.id
		resource.WithHost(),         // host.name
		resource.WithAttributes(
			semconv.ServiceName(cfg.AppName),
			semconv.ServiceVersion(cfg.AppVersion),
			semconv.DeploymentEnvironmentName(cfg.DeploymentEnv),
		),
	)
	if err != nil {
		logger.WarnContext(ctx,
			"partial resource detection; continuing with detected subset",
			slog.Any("error", err),
		)
	}

	// resource.New may return nil in catastrophic cases; degrade to a
	// minimal, always-safe resource.
	if res == nil {
		res = resource.NewSchemaless(
			semconv.ServiceName(cfg.AppName),
			semconv.ServiceVersion(cfg.AppVersion),
			semconv.DeploymentEnvironmentName(cfg.DeploymentEnv),
		)
	}

	return res, nil
}
