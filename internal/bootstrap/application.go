package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	"github.com/EnockYator/go-oauth/internal/infrastructure/observability/oteltracing"
	httpserver "github.com/EnockYator/go-oauth/internal/interfaces/http"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Application struct {
	db              *pgxpool.Pool
	tracerProvider  *sdktrace.TracerProvider
	server          *httpserver.Server
	shutdownTimeout time.Duration
}

func New(ctx context.Context) (*Application, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}

	if err := loadDevelopmentEnv(); err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}

	if err := config.ValidateConfig(*cfg); err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}

	logger, tracerProvider, err := initializeObservability(ctx, cfg)
	if err != nil {
		return nil, err
	}

	db, err := openDatabase(ctx, cfg.Database)
	if err != nil {
		_ = oteltracing.Shutdown(context.Background(), tracerProvider)
		return nil, err
	}

	authDeps, err := buildAuthDependencies(
		db,
		cfg,
		logger,
		otel.Tracer(cfg.App.AppName),
	)
	if err != nil {
		db.Close()
		_ = oteltracing.Shutdown(context.Background(), tracerProvider)

		return nil, err
	}

	server, err := buildHTTPServer(
		cfg,
		logger,
		tracerProvider,
		authDeps,
	)
	if err != nil {
		db.Close()
		_ = oteltracing.Shutdown(context.Background(), tracerProvider)

		return nil, err
	}

	return &Application{
		db:              db,
		tracerProvider:  tracerProvider,
		server:          server,
		shutdownTimeout: cfg.Server.ShutdownTimeout,
	}, nil
}

func (a *Application) Server() *httpserver.Server {
	if a == nil {
		return nil
	}

	return a.server
}

func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}

	var errs []error

	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown HTTP server: %w", err))
		}
	}

	if a.db != nil {
		a.db.Close()
	}

	if a.tracerProvider != nil {
		if err := oteltracing.Shutdown(ctx, a.tracerProvider); err != nil {
			errs = append(errs, fmt.Errorf("shutdown tracer provider: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (a *Application) ShutdownTimeout() time.Duration {
	if a == nil || a.shutdownTimeout <= 0 {
		return 30 * time.Second
	}

	return a.shutdownTimeout
}

func loadDevelopmentEnv() error {
	appEnv := os.Getenv("APP_ENV")

	if appEnv != "" && appEnv != "development" {
		return nil
	}

	err := godotenv.Load(".env.development")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("load development environment: %w", err)
	}

	return nil
}
