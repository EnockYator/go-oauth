package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	"github.com/EnockYator/go-oauth/internal/infrastructure/observability/oteltracing"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initializeObservability(ctx context.Context, cfg *config.Config) (*slog.Logger, *sdktrace.TracerProvider, error) {
	logger := oteltracing.NewLogger(cfg.App.AppEnv)

	tracerProvider, err := oteltracing.Init(ctx, oteltracing.Config{
		AppName:         cfg.App.AppName,
		AppVersion:      cfg.App.AppVersion,
		DeploymentEnv:   cfg.App.AppEnv,
		Protocol:        oteltracing.ProtocolGRPC,
		Endpoint:        cfg.OTel.Endpoint,
		Insecure:        cfg.App.AppEnv == "development",
		TLSCAFile:       cfg.OTel.TLSCAFile,
		TLSCertFile:     cfg.OTel.TLSCertFile,
		TLSKeyFile:      cfg.OTel.TLSKeyFile,
		TLSServerName:   cfg.OTel.TLSServerName,
		Headers:         cfg.OTel.Headers,
		Compression:     "gzip",
		ExportTimeout:   cfg.OTel.ExportTimeout,
		ShutdownTimeout: cfg.OTel.OtelShutdownTimeout,
		SamplingRatio:   cfg.OTel.SampleRatio,
	}, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize tracing: %w", err)
	}

	slog.SetDefault(logger)
	return logger, tracerProvider, nil
}
