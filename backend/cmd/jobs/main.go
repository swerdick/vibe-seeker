// Package main provides Lambda handlers for background sync jobs.
// Each Lambda function sets a JOB_NAME environment variable to select
// which job to run.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/pseudo/vibe-seeker/backend/internal/app"
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := configuration.LoadSSMConfig(context.Background()); err != nil {
		slog.Error("failed to load SSM config", "error", err)
		os.Exit(1)
	}

	cfg := configuration.NewConfig()

	// initCtx is only for setup. lambda.Start blocks forever, so a deferred
	// cancel on the init context would never run.
	initCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	pool, err := store.Connect(initCtx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc, err := app.New(cfg, pool)
	if err != nil {
		cancel()
		slog.Error("failed to build services", "error", err)
		os.Exit(1)
	}
	cancel()

	jobName := os.Getenv("JOB_NAME")
	slog.Info("starting job handler", "job", jobName)

	switch jobName {
	case "sync-venues":
		lambda.Start(func(ctx context.Context) error {
			result, err := svc.VenueSvc.SyncVenues(ctx)
			if err != nil {
				return fmt.Errorf("sync-venues: %w", err)
			}
			slog.Info("sync-venues complete", "result", result)
			return nil
		})
	case "sync-venue-vibes":
		lambda.Start(func(ctx context.Context) error {
			count, err := svc.VenueSvc.SyncVenueVibes(ctx)
			if err != nil {
				return fmt.Errorf("sync-venue-vibes: %w", err)
			}
			slog.Info("sync-venue-vibes complete", "venues_updated", count)
			return nil
		})
	case "tag-enrichment":
		lambda.Start(func(ctx context.Context) error {
			if err := svc.TagEnricher.EnrichStale(ctx); err != nil {
				return fmt.Errorf("tag-enrichment: %w", err)
			}
			return nil
		})
	default:
		slog.Error("unknown JOB_NAME", "job", jobName)
		os.Exit(1)
	}
}
