package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var sqlFiles embed.FS

// migration represents a single versioned migration with up and down SQL.
type migration struct {
	version int
	up      string
	down    string
}

// parseMigrations reads an fs.FS for files matching NNN_name.up.sql and
// NNN_name.down.sql, pairs them by version, and returns them sorted.
func parseMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("reading migration directory: %w", err)
	}

	byVersion := map[int]*migration{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}

		m, ok := byVersion[version]
		if !ok {
			m = &migration{version: version}
			byVersion[version] = m
		}

		switch {
		case strings.HasSuffix(name, ".up.sql"):
			m.up = string(content)
		case strings.HasSuffix(name, ".down.sql"):
			m.down = string(content)
		}
	}

	result := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		result = append(result, *m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].version < result[j].version
	})

	return result, nil
}

// withMigrationTx parses the embedded SQL files, begins a transaction,
// ensures the schema_migrations table exists, reads the current version,
// and passes everything to fn. It commits on success or rolls back on error.
func withMigrationTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx, migrations []migration, applied int) error) error {
	migrations, err := parseMigrations(sqlFiles)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	var applied int
	err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&applied)
	if err != nil {
		return fmt.Errorf("reading migration version: %w", err)
	}

	if err := fn(tx, migrations, applied); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Migrate applies all pending up migrations in order. It uses a
// schema_migrations table to track which versions have been applied.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return withMigrationTx(ctx, pool, func(tx pgx.Tx, migrations []migration, applied int) error {
		newCount := 0
		for _, m := range migrations {
			if m.version <= applied {
				continue
			}
			slog.Info("applying migration", "version", m.version)
			if _, err := tx.Exec(ctx, m.up); err != nil {
				return fmt.Errorf("applying migration %d: %w", m.version, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
				return fmt.Errorf("recording migration %d: %w", m.version, err)
			}
			newCount++
		}
		slog.Info("migrations complete", "total", len(migrations), "applied", newCount)
		return nil
	})
}

// MigrateDown rolls back applied migrations down to (but not including)
// the given target version. Pass 0 to roll back everything.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool, targetVersion int) error {
	return withMigrationTx(ctx, pool, func(tx pgx.Tx, migrations []migration, applied int) error {
		for _, m := range slices.Backward(migrations) {
			if m.version <= targetVersion || m.version > applied {
				continue
			}
			slog.Info("rolling back migration", "version", m.version)
			if _, err := tx.Exec(ctx, m.down); err != nil {
				return fmt.Errorf("rolling back migration %d: %w", m.version, err)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.version); err != nil {
				return fmt.Errorf("removing migration record %d: %w", m.version, err)
			}
		}
		slog.Info("rollback complete", "target_version", targetVersion)
		return nil
	})
}
