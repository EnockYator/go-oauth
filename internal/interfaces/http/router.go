package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/health"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/root"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/jwt"
	"github.com/jackc/pgx/v5/pgxpool"

	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/otel/trace"
)

// RouterConfig contains everything required to construct the HTTP router.
type RouterConfig struct {
	DB             *pgxpool.Pool
	Logger         *slog.Logger
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

	// Exact-match root: only matches "GET /", nothing else.
	publicMux.HandleFunc("GET /{$}", root.Root)
	// Explicit health and swagger routes.
	publicMux.HandleFunc("/healthz/", health.Healthz)
	publicMux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Fallback: any unmatched path returns 404.
	// Registered last because ServeMux resolves by specificity, not order —
	// but keeping it last makes the intent readable.
	publicMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

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
			p := r.URL.Path
			return strings.HasPrefix(p, "/health") ||
				strings.HasPrefix(p, "/swagger")
		}),
	}

	if cfg.TracerProvider != nil {
		traceOpts = append(traceOpts, middleware.WithTracerProvider(cfg.TracerProvider))
	}

	// ------------------------------------------------------------
	// Middleware Chain Construction
	// ------------------------------------------------------------

	// Middleware order (outer → inner):
	// 1. Tracing  →  starts server span, injects it into r.Context()
	// 2. Recovery  →  catch panics
	// 3. Request ID  →  add unique ID to context
	// 4. CORS  →  handle cross-origin requests
	// 5. Rate Limiting  →  throttle excessive requests
	// 6. Logging  →  request/response logging
	// 7. Timeout  →  per-request deadline enforcement
	// 8. Router  →  actual request handling
	// Trace → Recovery → RequestID → CORS → RateLimit → Logger → Timeout → Router
	handler := middleware.NewTraceMiddleware(traceOpts...)(
		middleware.RequestIDMiddleware(
			middleware.RecoveryMiddleware()(
				corsMiddleware(
					rateLimiter.RateLimitMiddleware(
						middleware.LoggerMiddleware(cfg.Logger)(
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
