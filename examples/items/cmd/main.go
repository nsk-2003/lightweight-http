// Purpose: Entry point for the standalone items-server example; demonstrates complete
// framework wiring: DI, middleware chain, route groups, and graceful shutdown.

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/lightweight-http/examples/items"
	"github.com/example/lightweight-http/pkg/di"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

func main() {
	port := envOr("PORT", "8080")
	debug := envOr("DEBUG", "false") == "true"
	logger := buildLogger(envOr("LOG_LEVEL", "info"))

	container := buildContainer(logger)
	collector := observability.NewInProcessCollector()
	ro := buildRouter(container, collector, logger, debug)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           ro,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	} else {
		logger.Info("server stopped cleanly")
	}
}

func buildLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func buildContainer(logger *slog.Logger) *di.Container {
	c := di.New()
	if err := c.Register(items.StoreKey, func(_ *di.Resolver) (any, error) {
		return items.NewItemStore(), nil
	}, di.Singleton); err != nil {
		logger.Error("DI registration failed", "error", err)
		os.Exit(1)
	}
	return c
}

func buildRouter(c *di.Container, col *observability.InProcessCollector, logger *slog.Logger, debug bool) *router.Router {
	ro := router.New()
	ro.Use(
		observability.RequestID(),
		observability.LoggingMiddleware(logger),
		observability.MetricsMiddleware(col),
		middleware.Recovery(logger),
	)
	v1 := ro.Group("/api/v1")
	items.RegisterRoutes(v1, c, col, debug)
	return ro
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
