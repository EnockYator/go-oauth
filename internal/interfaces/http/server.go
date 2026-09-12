package http

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/EnockYator/go-oauth/internal/config"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"
	"go.opentelemetry.io/otel/trace"
)

// Server owns the HTTP server and its HTTP-layer dependencies.
//
// Server is responsible for:
//   - constructing the HTTP router
//   - configuring net/http.Server
//   - starting the HTTP listener
//   - gracefully shutting down the HTTP server
//
// Application lifecycle and OS signal handling belong to main.
type Server struct {
	cfg *config.AppConfig
	db  *sql.DB

	logger         *slog.Logger
	tracerProvider trace.TracerProvider

	cors           middleware.CORSConfig
	rateLimiter    middleware.RateLimiterConfig
	requestTimeout time.Duration

	httpServer *http.Server
	router     *Router
}

// ServerOptions contains dependencies required by the HTTP server.
//
// Dependencies are injected rather than constructed inside Server so that
// infrastructure concerns remain separated and the server remains easy
// to test.
type ServerOptions struct {
	Logger         *slog.Logger
	TracerProvider trace.TracerProvider

	CORS           middleware.CORSConfig
	RateLimiter    middleware.RateLimiterConfig
	RequestTimeout time.Duration
}

// NewServer constructs the HTTP server and its router.
//
// NewServer performs dependency wiring but does not start listening on a
// network socket.
func NewServer(
	cfg *config.AppConfig,
	db *sql.DB,
	opts ServerOptions,
) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("configuration must not be nil")
	}

	if db == nil {
		return nil, errors.New("database must not be nil")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	router, err := NewRouter(RouterConfig{
		DB:             db,
		Logger:         logger,
		TracerProvider: opts.TracerProvider,
		CORS:           opts.CORS,
		RateLimiter:    opts.RateLimiter,
		RequestTimeout: opts.RequestTimeout,
	})
	if err != nil {
		return nil, errors.Join(
			errors.New("create HTTP router"),
			err,
		)
	}

	port := strconv.Itoa(cfg.Server.Port)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:            router.Handler(),
		ReadTimeout:       cfg.Server.ReadTimeout * time.Second,
		ReadHeaderTimeout: cfg.Server.ReadTimeout * time.Second,
		WriteTimeout:      cfg.Server.WriteTimeout * time.Second,
		IdleTimeout:       cfg.Server.IdleTimeout * time.Second,
	}

	return &Server{
		cfg:            cfg,
		db:             db,
		logger:         logger,
		tracerProvider: opts.TracerProvider,

		cors:           opts.CORS,
		rateLimiter:    opts.RateLimiter,
		requestTimeout: opts.RequestTimeout,

		httpServer: httpServer,
		router:     router,
	}, nil
}

// Start starts the HTTP server.
//
// Start blocks until the HTTP server stops or encounters an error.
// It does not handle OS signals. Application-level signal handling belongs
// to main.
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting")
	s.logger.Info(
		"HTTP server configuration",
		slog.String("address:", s.httpServer.Addr),
		slog.String("environment", s.cfg.AppEnv),
		slog.Duration("read_timeout", time.Duration(s.cfg.Server.ReadTimeout.Seconds())),
		slog.Duration("read_header_timeout", time.Duration(s.cfg.Server.ReadHeaderTimeout.Seconds())),
		slog.Duration("write_timeout", time.Duration(s.cfg.Server.WriteTimeout.Seconds())),
		slog.Duration("idle_timeout", time.Duration(s.cfg.Server.IdleTimeout.Seconds())),
	)

	err := s.httpServer.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

// Shutdown gracefully shuts down the HTTP server.
//
// Existing connections are allowed to complete until the supplied context
// expires.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("HTTP server shutting down")

	err := s.httpServer.Shutdown(ctx)

	if err != nil {
		s.logger.Error(
			"HTTP server shutdown failed",
			slog.Any("error", err),
		)

		return err
	}

	// Router-owned resources such as rate limiter cleanup goroutines should
	// be released after the HTTP server has stopped accepting/serving
	// requests.
	if s.router != nil {
		s.router.Close()
	}

	s.logger.Info("HTTP server stopped")

	return nil
}