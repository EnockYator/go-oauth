package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/EnockYator/go-oauth/docs"
	"github.com/EnockYator/go-oauth/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx)
	if err != nil {
		slog.Error("application initialization failed", slog.Any("error", err))
		os.Exit(1)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Server().Start()
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			slog.Error("HTTP server stopped unexpectedly", slog.Any("error", err))
			shutdown(ctx, app)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.ShutdownTimeout())
		defer cancel()

		if err := app.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", slog.Any("error", err))
			os.Exit(1)
		}
	}
}

func shutdown(ctx context.Context, app *bootstrap.Application) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.ShutdownTimeout())
	defer cancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(ctx, "application shutdown failed", slog.Any("error", err))
	}
}