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

	"github.com/EnockYator/go-oauth/internal/config"
	"github.com/EnockYator/go-oauth/internal/infrastructure/database/postgres"
	"github.com/EnockYator/go-oauth/internal/infrastructure/observability/tracing"
	httpserver "github.com/EnockYator/go-oauth/internal/interfaces/http"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"
	"github.com/joho/godotenv"
)

const (
	serviceName    = "go-oauth"
	serviceVersion = "1.0.0"

	defaultShutdownTimeout = 10 * time.Second
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
	// 3. Create application context.
	// ---------------------------------------------------------------------

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// ---------------------------------------------------------------------
	// 4. Initialize tracing.
	// ---------------------------------------------------------------------

	tracerProvider, err := tracing.Init(
		signalCtx,
		tracing.Config{
			ServiceName:     serviceName,
			ServiceVersion:   serviceVersion,
			DeploymentEnv:    cfg.App.AppEnv,
			SamplingRatio:   0.10,
			OTLPEndpoint:    "",
			OTLPHeaders:     "",
			ShutdownTimeout: 5 * time.Second,
		},
		logger,
	)
	if err != nil {
		return errors.Join(
			errors.New("initialize tracing"),
			err,
		)
	}

	// Tracing must be shut down before the application exits so that
	// pending spans have an opportunity to be exported.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := tracing.Shutdown(shutdownCtx, tracerProvider); err != nil {
			logger.Error(
				"failed to shutdown tracing",
				slog.Any("error", err),
			)
		}
	}()

	logger.Info("tracing initialized")

	// ---------------------------------------------------------------------
	// 5. Initialize database.
	// ---------------------------------------------------------------------

	db, err := postgres.New(cfg.Database)
	if err != nil {
		return errors.Join(
			errors.New("initialize database"),
			err,
		)
	}

	// Database cleanup belongs here because main owns the database
	// lifecycle.
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error(
				"failed to close database",
				slog.Any("error", err),
			)
		}
	}()

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

	logger.Info("application shutdown initiated")

	if err := server.Shutdown(shutdownCtx); err != nil {
		return errors.Join(
			errors.New("shutdown HTTP server"),
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

// shutdownSignal returns a useful signal description.
//
// signal.NotifyContext does not expose the actual signal directly, so this
// function keeps logging simple and avoids maintaining another signal channel.
func shutdownSignal(ctx context.Context) string {
	if ctx.Err() == context.Canceled {
		return "context canceled"
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "context deadline exceeded"
	}

	return "operating-system signal"
}