package bootstrap

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	httpserver "github.com/EnockYator/go-oauth/internal/interfaces/http"
	// "github.com/EnockYator/go-oauth/internal/interfaces/http/cookie"
	authhandler "github.com/EnockYator/go-oauth/internal/interfaces/http/handler/auth"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/middleware"
	"go.opentelemetry.io/otel/trace"
)

func buildHTTPServer(
	cfg *config.Config,
	logger *slog.Logger,
	tracerProvider trace.TracerProvider,
	deps *AuthDependencies,
) (*httpserver.Server, error) {
	handler := authhandler.NewHandler(
		deps.InitiateLogin,
		deps.HandleCallback,
		deps.Logout,
		deps.SessionCookie,
		deps.StateCookie,
	)

	server, err := httpserver.NewServer(
		cfg,
		httpserver.ServerOptions{
			Logger:         logger,
			TracerProvider: tracerProvider,
			CORS: middleware.CORSConfig{
				AllowedOrigins:   []string{"http://localhost:3000"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			RateLimiter: middleware.RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             20,
				CleanupInterval:   10 * time.Minute,
				TrustProxy:        cfg.App.AppEnv != "development",
			},
			RequestTimeout: 30 * time.Second,
		},
		handler,
		middleware.SessionValidator(deps.ValidateSession),
		deps.SessionCookie,
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP server: %w", err)
	}
	return server, nil
}
