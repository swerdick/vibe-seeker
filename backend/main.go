package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/migrations"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/webserver"
)

func main() {
	if err := configuration.LoadSSMConfig(context.Background()); err != nil {
		slog.Error("failed to load SSM config", "error", err)
		os.Exit(1)
	}

	cfg := configuration.NewConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: webserver.ParseLogLevel(cfg.LogLevel),
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	otelShutdown, err := observability.Init(ctx, cfg.AppName, cfg.Environment, cfg.OtelEnabled)
	if err != nil {
		slog.Error("failed to initialize telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			slog.Error("telemetry shutdown error", "error", err)
		}
	}()

	startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startupCancel()

	pool, err := store.Connect(startupCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Acquire an advisory lock to prevent concurrent migration races across
	// Lambda cold starts. Wrapped in a closure so the defer always runs —
	// leaking a locked session back into the pool would block every future
	// cold start on pg_advisory_lock(1).
	if err := func() error {
		conn, err := pool.Acquire(startupCtx)
		if err != nil {
			return err
		}

		lockAcquired := false
		defer func() {
			if lockAcquired {
				if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock(1)"); err != nil {
					// If unlock fails, the session is poisoned — hijack the
					// connection out of the pool and close it so the lock is
					// released when Postgres tears down the session.
					slog.Error("failed to release migration advisory lock; closing connection", "error", err)
					if closeErr := conn.Hijack().Close(context.Background()); closeErr != nil {
						slog.Error("failed to close hijacked connection", "error", closeErr)
					}
					return
				}
			}
			conn.Release()
		}()

		if _, err := conn.Exec(startupCtx, "SELECT pg_advisory_lock(1)"); err != nil {
			return err
		}
		lockAcquired = true

		return migrations.Migrate(startupCtx, pool)
	}(); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	server, err := webserver.New(cfg, pool)
	if err != nil {
		slog.Error("failed to build server", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("starting server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
}
