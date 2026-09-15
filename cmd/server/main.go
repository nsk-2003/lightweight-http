// Purpose: Production server entry point; reads configuration from environment variables,
// wires the DI container, middleware chain, and route group, then starts the HTTP server
// with graceful shutdown on SIGINT/SIGTERM.

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
	cfg := loadConfig()
	logger := newLogger(cfg.logLevel)

	container := newContainer(logger)
	collector := observability.NewInProcessCollector()
	handler := newHandler(container, collector, logger, cfg.debug)

	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           handler,
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

// config holds all server configuration resolved from environment variables.
type config struct {
	port     string
	logLevel string
	debug    bool
}

func loadConfig() config {
	return config{
		port:     envOr("PORT", "8080"),
		logLevel: envOr("LOG_LEVEL", "info"),
		debug:    os.Getenv("DEBUG") == "true",
	}
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func newContainer(logger *slog.Logger) *di.Container {
	c := di.New()
	if err := c.Register(items.StoreKey, func(_ *di.Resolver) (any, error) {
		return items.NewItemStore(), nil
	}, di.Singleton); err != nil {
		logger.Error("DI registration failed", "error", err)
		os.Exit(1)
	}
	return c
}

func newHandler(c *di.Container, col *observability.InProcessCollector, logger *slog.Logger, debug bool) http.Handler {
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
