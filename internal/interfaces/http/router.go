package http

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/EnockYator/go-oauth/internal/domain/auth/infrastructure/jwt"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/health"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/root"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"

	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/otel/trace"
)

// RouterConfig contains everything required to construct the HTTP router.
type RouterConfig struct {
	DB             *sql.DB
	Logger *slog.Logger
	JWTValidator   jwt.TokenValidator
	TracerProvider trace.TracerProvider
	CORS           middleware.CORSConfig
	RateLimiter    middleware.RateLimiterConfig
	RequestTimeout time.Duration
}

// Router owns the HTTP handler and resources that require lifecycle
// management.
type Router struct {
	handler     http.Handler
	rateLimiter *middleware.RateLimiter
}

// NewRouter constructs and validates the complete HTTP middleware stack.
func NewRouter(cfg RouterConfig) (*Router, error) {
	// Validate critical dependencies
	if cfg.DB == nil {
		return nil, fmt.Errorf("http router: database connection required")
	}
	// if cfg.JWTValidator == nil {
	// 	return nil, fmt.Errorf("http router: JWT validator required")
	// }

	// Initialize middleware components
	corsMiddleware, err := middleware.NewCORS(cfg.CORS)
	if err != nil {
		return nil, fmt.Errorf("http router: initialize CORS: %w", err)
	}

	rateLimiter, err := middleware.NewRateLimiter(cfg.RateLimiter)
	if err != nil {
		return nil, fmt.Errorf("http router: initialize rate limiter: %w", err)
	}

	timeoutMiddleware, err := middleware.NewTimeout(cfg.RequestTimeout)
	if err != nil {
		rateLimiter.Close()
		return nil, fmt.Errorf("http router: initialize request timeout: %w", err)
	}

	// ------------------------------------------------------------
	// Route Definitions
	// ------------------------------------------------------------

	// Public routes (no authentication required)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/", root.RootHandler)
	publicMux.HandleFunc("/health", health.Health)
	publicMux.HandleFunc("/health/live", health.Live)
	publicMux.HandleFunc("/health/ready", health.Ready(cfg.DB))
	publicMux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Protected routes (require authentication)
	protectedMux := http.NewServeMux()
	// Register protected API endpoints here
	// protectedMux.HandleFunc("/api/v1/photos", photoHandler.List)

	protectedHandler := middleware.AuthMiddleware(cfg.JWTValidator)(
		middleware.TenantMiddleware(protectedMux),
	)

	// Root router combines public and protected routes
	rootMux := http.NewServeMux()
	rootMux.Handle("/api/", protectedHandler) // Protected API routes
	rootMux.Handle("/", publicMux)            // Public routes

	traceOpts := []middleware.TraceMiddlewareOption{
		middleware.WithServiceName("go-oauth"),
		middleware.WithFilter(func(r *http.Request) bool {
			return strings.HasPrefix(r.URL.Path, "/health") ||
				strings.HasPrefix(r.URL.Path, "/swagger")
		}),
	}

	if cfg.TracerProvider != nil {
		traceOpts = append(traceOpts, middleware.WithTracerProvider(cfg.TracerProvider))
	}

	// ------------------------------------------------------------
	// Middleware Chain Construction
	// ------------------------------------------------------------

	// Middleware order (outer → inner):
	// 1. Recovery (catch panics)
	// 2. Request ID (add unique ID to context)
	// 3. Tracing (distributed tracing)
	// 4. CORS (handle cross-origin requests)
	// 5. Rate Limiting (throttle excessive requests)
	// 6. Logging (request/response logging)
	// 7. Timeout (request deadline enforcement)
	// 8. Router (actual request handling)
	// Recovery → RequestID → Trace → CORS → RateLimit → Logger → Timeout → Router
	handler := middleware.RecoveryMiddleware()(
		middleware.RequestIDMiddleware(
			middleware.NewTraceMiddleware(traceOpts...)(
				corsMiddleware(
					rateLimiter.RateLimitMiddleware(
						middleware.LoggerMiddleware()(
							timeoutMiddleware(
								rootMux,
							),
						),
					),
				),
			),
		),
	)

	return &Router{
		handler:     handler,
		rateLimiter: rateLimiter,
	}, nil
}

// Handler returns the HTTP handler used by http.Server.
func (r *Router) Handler() http.Handler {
	return r.handler
}

// Close releases resources owned by the router.
func (r *Router) Close() {
	if r != nil && r.rateLimiter != nil {
		r.rateLimiter.Close()
	}
}