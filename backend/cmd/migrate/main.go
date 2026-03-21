package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/migrations"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

func main() {
	down := flag.Bool("down", false, "roll back migrations instead of applying them")
	target := flag.Int("to", 0, "target version for rollback (used with -down)")
	flag.Parse()

	cfg := configuration.NewConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if *down {
		if err := migrations.MigrateDown(ctx, pool, *target); err != nil {
			slog.Error("rollback failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("rollback complete")
	} else {
		if err := migrations.Migrate(ctx, pool); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		fmt.Println("migrations complete")
	}
}
