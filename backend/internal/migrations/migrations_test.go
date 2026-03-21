package migrations

import (
	"testing"
	"testing/fstest"
)

func TestParseMigrations_SortsAndPairs(t *testing.T) {
	fsys := fstest.MapFS{
		"002_create_venues.up.sql":   {Data: []byte("CREATE TABLE venues (id TEXT)")},
		"002_create_venues.down.sql": {Data: []byte("DROP TABLE venues")},
		"001_create_users.up.sql":    {Data: []byte("CREATE TABLE users (id TEXT)")},
		"001_create_users.down.sql":  {Data: []byte("DROP TABLE users")},
	}

	migrations, err := parseMigrations(fsys)
	if err != nil {
		t.Fatalf("parseMigrations failed: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	if migrations[0].version != 1 {
		t.Errorf("first migration version = %d, want 1", migrations[0].version)
	}
	if migrations[1].version != 2 {
		t.Errorf("second migration version = %d, want 2", migrations[1].version)
	}

	if migrations[0].up != "CREATE TABLE users (id TEXT)" {
		t.Errorf("unexpected up SQL: %s", migrations[0].up)
	}
	if migrations[0].down != "DROP TABLE users" {
		t.Errorf("unexpected down SQL: %s", migrations[0].down)
	}
}

func TestParseMigrations_SkipsNonSQL(t *testing.T) {
	fsys := fstest.MapFS{
		"001_create_users.up.sql": {Data: []byte("CREATE TABLE users (id TEXT)")},
		"migrations.go":           {Data: []byte("package migrations")},
		"README.md":               {Data: []byte("# Migrations")},
	}

	migrations, err := parseMigrations(fsys)
	if err != nil {
		t.Fatalf("parseMigrations failed: %v", err)
	}

	if len(migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(migrations))
	}
}

func TestParseMigrations_Empty(t *testing.T) {
	fsys := fstest.MapFS{}

	migrations, err := parseMigrations(fsys)
	if err != nil {
		t.Fatalf("parseMigrations failed: %v", err)
	}

	if len(migrations) != 0 {
		t.Fatalf("expected 0 migrations, got %d", len(migrations))
	}
}
