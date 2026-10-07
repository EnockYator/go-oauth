package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/cookie"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/auth"
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
	cfg *config.Config

	logger *slog.Logger

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
	cfg *config.Config,
	opts ServerOptions,
	authHandler *auth.Handler,
	validateSession middleware.SessionValidator,
	sessionCookieCfg cookie.SessionConfig,

) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("configuration must not be nil")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	router, err := NewRouter(RouterConfig{
		Logger:         logger,
		TracerProvider: opts.TracerProvider,
		CORS:           opts.CORS,
		RateLimiter:    opts.RateLimiter,
		RequestTimeout: opts.RequestTimeout,

		// Auth wiring
		AuthHandler:         authHandler,
		SessionValidator:    validateSession,
		SessionCookieConfig: sessionCookieCfg,
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
		Handler:           router.Handler(),
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	return &Server{
		cfg:    cfg,
		logger: logger,

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
		slog.String("address", s.httpServer.Addr),
		slog.String("environment", s.cfg.App.AppEnv),
		slog.Duration("read_timeout", time.Duration(s.cfg.Server.ReadTimeout)),
		slog.Duration("read_header_timeout", time.Duration(s.cfg.Server.ReadHeaderTimeout)),
		slog.Duration("write_timeout", time.Duration(s.cfg.Server.WriteTimeout)),
		slog.Duration("idle_timeout", time.Duration(s.cfg.Server.IdleTimeout)),
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
