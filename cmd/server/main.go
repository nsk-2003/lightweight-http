// Purpose: Production server entry point — constructs the DI container,
// registers services, builds the middleware chain, mounts routes via
// examples/items, and starts an http.Server with all timeouts and graceful
// shutdown on SIGINT/SIGTERM (ADR-005, ADR-011).
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
	"github.com/example/lightweight-http/pkg/observability"
)

func main() {
	port := envOrDefault("LWHTTP_PORT", "8080")
	logLevel := parseLogLevel(envOrDefault("LWHTTP_LOG_LEVEL", "info"))
	debug := os.Getenv("LWHTTP_DEBUG") == "true"

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(log)

	// Build the DI container and register application services.
	ctr := di.NewContainer()
	if err := ctr.Register(items.StoreKey, func(r di.Resolver) (any, error) {
		return items.NewStore(), nil
	}, di.Singleton); err != nil {
		log.Error("DI registration failed", slog.String("key", items.StoreKey), slog.Any("error", err))
		os.Exit(1)
	}

	rec := &observability.Recorder{}
	h := items.NewHandler(ctr, rec, log, debug)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Shut down gracefully on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("server starting", slog.String("addr", srv.Addr), slog.Bool("debug", debug))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server listen error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	log.Info("shutting down gracefully")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("shutdown error", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("server stopped")
}

// envOrDefault returns the environment variable value for name, or fallback
// when the variable is unset or empty.
func envOrDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// parseLogLevel maps the LWHTTP_LOG_LEVEL string to a slog.Level.
// Unrecognized values default to Info.
func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
