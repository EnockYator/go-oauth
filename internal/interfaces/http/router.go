package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/cookie"
	authHandler "github.com/EnockYator/go-oauth/internal/interfaces/http/handler/auth"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/health"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/handler/root"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"

	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/otel/trace"
)

// RouterConfig contains everything required to construct the HTTP router.
type RouterConfig struct {
	Logger         *slog.Logger
	TracerProvider trace.TracerProvider

	CORS           middleware.CORSConfig
	RateLimiter    middleware.RateLimiterConfig
	RequestTimeout time.Duration

	// Auth wiring. All three are required when any /auth route is
	// registered; NewRouter validates this.
	AuthHandler         *authHandler.Handler
	SessionValidator    middleware.SessionValidator
	SessionCookieConfig cookie.SessionConfig
}

// Router owns the HTTP handler and resources that require lifecycle
// management.
type Router struct {
	handler     http.Handler
	rateLimiter *middleware.RateLimiter
}

// NewRouter constructs and validates the complete HTTP middleware stack.
func NewRouter(cfg RouterConfig) (*Router, error) {
	if cfg.AuthHandler == nil {
		return nil, fmt.Errorf("http router: auth handler required")
	}
	if cfg.SessionValidator == nil {
		return nil, fmt.Errorf("http router: session validator required")
	}

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

	// Protected routes (require authentication)
	protectedMux := http.NewServeMux()

	// authMW wraps handlers that require an authenticated session.
	authMW := middleware.AuthMiddleware(
		cfg.SessionValidator,
		cfg.SessionCookieConfig,
	)(protectedMux)

	protectedMux.HandleFunc("GET /api/me", cfg.AuthHandler.Me)
	protectedMux.HandleFunc("GET /api/auth/me", cfg.AuthHandler.Me)

	// Public routes (no authentication required)
	publicMux := http.NewServeMux()

	// Exact-match root: only matches "GET /", nothing else.
	publicMux.HandleFunc("GET /{$}", root.Root)
	publicMux.HandleFunc("GET /healthz", health.Healthz)
	publicMux.HandleFunc("GET /healthz/", health.Healthz)
	publicMux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Public auth endpoints.
	publicMux.HandleFunc("GET /auth/login/{provider}", cfg.AuthHandler.Login)
	publicMux.HandleFunc("GET /auth/callback/{provider}", cfg.AuthHandler.Callback)
	publicMux.HandleFunc("POST /auth/logout", cfg.AuthHandler.Logout)

	// Fallback: any unmatched path returns 404.
	// Registered last because ServeMux resolves by specificity, not order —
	// but keeping it last makes the intent readable.
	publicMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	// Root router combines public and protected routes
	rootMux := http.NewServeMux()
	rootMux.Handle("/api/", authMW) // Protected API routes
	rootMux.Handle("/", publicMux)  // Public routes

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
	// 1. Tracing  → starts the server span.
	// 2. Request ID → adds a server-generated request ID.
	// 3. Logging → observes every response, including rejected requests.
	// 4. Recovery → catches panics from the remaining HTTP stack.
	// 5. CORS → handles cross-origin policy and preflight requests.
	// 6. Rate limiting → throttles excessive requests.
	// 7. Timeout → enforces the per-request deadline.
	// 8. Router → executes the actual endpoint.
	// Trace → RequestID → Logger → Recovery → CORS → RateLimit → Timeout → Router
	handler := middleware.NewTraceMiddleware(traceOpts...)(
		middleware.RequestIDMiddleware(
			middleware.LoggerMiddleware(cfg.Logger)(
				middleware.RecoveryMiddleware()(
					corsMiddleware(
						rateLimiter.RateLimitMiddleware(
							timeoutMiddleware(rootMux),
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
