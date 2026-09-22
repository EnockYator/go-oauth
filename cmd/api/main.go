package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/EnockYator/go-oauth/docs"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	"github.com/EnockYator/go-oauth/internal/infrastructure/database/postgres"
	"github.com/EnockYator/go-oauth/internal/infrastructure/observability/oteltracing"
	httpserver "github.com/EnockYator/go-oauth/internal/interfaces/http"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"
	"github.com/joho/godotenv"
)

const (
	serviceName    = "go-oauth"
	serviceVersion = "1.0.0"

	defaultShutdownTimeout = 30 * time.Second
)

func main() {
	// Initialize the logger as early as possible so that every subsequent
	// initialization failure is logged consistently.
	logger := newLogger()
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error(
			"application terminated with error",
			slog.Any("error", err),
		)

		os.Exit(1)
	}
}

// run contains the application composition root.
//
// Its responsibility is to construct the application's infrastructure,
// wire dependencies together, start the application, and orchestrate
// graceful shutdown.
func run(logger *slog.Logger) error {
	// ---------------------------------------------------------------------
	// 1. Load development environment variables.
	// ---------------------------------------------------------------------

	appEnv := os.Getenv("APP_ENV")
	logger = oteltracing.NewLogger(appEnv)
	slog.SetDefault(logger)

	if err := loadDevelopmentEnv(); err != nil {
		return err
	}

	// ---------------------------------------------------------------------
	// 2. Load configuration.
	// ---------------------------------------------------------------------

	cfg, err := config.Load()
	if err != nil {
		return errors.Join(
			errors.New("load configuration"),
			err,
		)
	}

	// Never dereference cfg until Load has succeeded.
	if err := config.ValidateConfig(*cfg); err != nil {
		return errors.Join(
			errors.New("validate configuration"),
			err,
		)
	}

	logger.Info(
		"configuration loaded",
		slog.String("environment", cfg.App.AppEnv),
	)

	// ---------------------------------------------------------------------
	// 3. Create application context (cancelled on SIGINT/SIGTERM).
	// ---------------------------------------------------------------------

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// ---------------------------------------------------------------------
	// 4. Initialize oteltracing.
	//
	//    Note: oteltracing must be initialized BEFORE the HTTP server so
	//    that any instrumented dependency (otelhttp, otelpgx, ...)
	//    resolves the global TracerProvider at construction time.
	// ---------------------------------------------------------------------

	oteltracingCfg := oteltracing.Config{
		AppName:     serviceName,
		AppVersion:  serviceVersion,
		DeploymentEnv:   cfg.App.AppEnv,

		Protocol: oteltracing.ProtocolGRPC, // or ProtocolHTTP
		Endpoint: cfg.OTel.Endpoint,    // empty -> OTEL_EXPORTER_OTLP_ENDPOINT

		// Production should always use TLS. Set Insecure=true only in
		// local development or when the collector is on a trusted
		// private network.
		Insecure: cfg.App.AppEnv == "development",
		TLSCAFile:   cfg.OTel.TLSCAFile,   // optional
		TLSCertFile: cfg.OTel.TLSCertFile, // optional (mTLS)
		TLSKeyFile:  cfg.OTel.TLSKeyFile,  // optional (mTLS)

		Headers:     cfg.OTel.Headers, // e.g. {"x-honeycomb-team": "..."}
		Compression: "gzip",

		ExportTimeout:   10 * time.Second,
		ShutdownTimeout: 5 * time.Second,

		SamplingRatio: cfg.OTel.SampleRatio, // e.g. 0.10 in prod, 1.0 in dev
	}

	tracerProvider, err := oteltracing.Init(signalCtx, oteltracingCfg, logger)
	if err != nil {
		return errors.Join(errors.New("initialize oteltracing"), err)
	}

	// oteltracing must outlive the HTTP server: flush spans AFTER the server
	// stops serving requests.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			oteltracingCfg.ShutdownTimeout,
		)
		defer cancel()

		if err := oteltracing.Shutdown(shutdownCtx, tracerProvider); err != nil {
			logger.Error(
				"failed to shutdown oteltracing",
				slog.Any("error", err),
			)
		}
	}()

	logger.InfoContext(signalCtx, "oteltracing initialized")

	// ---------------------------------------------------------------------
	// 5. Initialize database.
	// ---------------------------------------------------------------------

	db, err := postgres.New(signalCtx, cfg.Database)
	if err != nil {
		return errors.Join(
			errors.New("initialize database"),
			err,
		)
	}

	// Database cleanup belongs here because main owns the database
	// lifecycle.
	defer db.Close()

	logger.Info("database initialized")

	// ---------------------------------------------------------------------
	// 6. Construct HTTP server.
	// ---------------------------------------------------------------------

	server, err := httpserver.NewServer(
		cfg,
		db,
		httpserver.ServerOptions{
			Logger:         logger,
			// JWTValidator:   tokenValidator,
			TracerProvider: tracerProvider,

			CORS: middleware.CORSConfig{
				AllowedOrigins: []string{
					"http://localhost:3000",
				},
				AllowedMethods: []string{
					http.MethodGet,
					http.MethodPost,
					http.MethodPut,
					http.MethodPatch,
					http.MethodDelete,
					http.MethodOptions,
				},
				AllowedHeaders: []string{
					"Authorization",
					"Content-Type",
					"X-Request-ID",
				},
				AllowCredentials: false,
				MaxAge:           3600,
			},

			RateLimiter: middleware.RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             20,
				CleanupInterval:   10 * time.Minute,
				TrustProxy:        false,
			},

			RequestTimeout: 30 * time.Second,
		},
	)
	if err != nil {
		return errors.Join(
			errors.New("create HTTP server"),
			err,
		)
	}

	// ---------------------------------------------------------------------
	// 7. Start HTTP server.
	// ---------------------------------------------------------------------

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.Start()
	}()

	logger.Info("application started")

	// ---------------------------------------------------------------------
	// 8. Wait for either:
	//
	//    a. an operating-system shutdown signal
	//    b. an unrecoverable HTTP server error
	// ---------------------------------------------------------------------

	select {
	case err := <-serverErr:
		if err != nil {
			return errors.Join(
				errors.New("HTTP server stopped unexpectedly"),
				err,
			)
		}

		return nil

	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	}

	// ---------------------------------------------------------------------
	// 9. Gracefully shut down HTTP server.
	// ---------------------------------------------------------------------

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		defaultShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return errors.Join(
			errors.New("failed to shutdown HTTP server"),
			err,
		)
	}

	logger.Info("application stopped")

	return nil
}

// newLogger creates the application's structured logger.
//
// The default logger is configured once at the composition root and is
// subsequently available through log/slog throughout the application.
func newLogger() *slog.Logger {
	handler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
	)

	return slog.New(handler)
}

// loadDevelopmentEnv loads the development .env file only when the
// application is running in development.
//
// Existing environment variables are not overwritten by godotenv.Load.
func loadDevelopmentEnv() error {
	appEnv := os.Getenv("APP_ENV")

	if appEnv != "" && appEnv != "development" {
		return nil
	}

	if err := godotenv.Load(".env.development"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The environment may have been supplied by the shell,
			// container runtime, IDE, CI/CD, etc.
			return nil
		}

		return errors.Join(
			errors.New("load .env.development"),
			err,
		)
	}

	return nil
}