package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pseudo/vibe-seeker/backend/internal/migrations"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := migrations.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestIntegration_UpsertUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s, _ := NewUserStore(pool)

	err := s.UpsertUser(ctx, "test-user-1", "Test User", "token-a", "refresh-a", 9999)
	if err != nil {
		t.Fatalf("first UpsertUser: %v", err)
	}

	// Upsert again with updated values.
	err = s.UpsertUser(ctx, "test-user-1", "Updated Name", "token-b", "refresh-b", 8888)
	if err != nil {
		t.Fatalf("second UpsertUser: %v", err)
	}

	tokens, err := s.GetTokens(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("GetTokens: %v", err)
	}
	if tokens.AccessToken != "token-b" {
		t.Errorf("AccessToken = %q, want token-b", tokens.AccessToken)
	}
	if tokens.TokenExpiry != 8888 {
		t.Errorf("TokenExpiry = %d, want 8888", tokens.TokenExpiry)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = 'test-user-1'`)
	})
}

func TestIntegration_GetTokens(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s, _ := NewUserStore(pool)

	_ = s.UpsertUser(ctx, "test-tokens-1", "Token User", "access-123", "refresh-456", 1234)

	tokens, err := s.GetTokens(ctx, "test-tokens-1")
	if err != nil {
		t.Fatalf("GetTokens: %v", err)
	}
	if tokens.AccessToken != "access-123" {
		t.Errorf("AccessToken = %q, want access-123", tokens.AccessToken)
	}
	if tokens.RefreshToken != "refresh-456" {
		t.Errorf("RefreshToken = %q, want refresh-456", tokens.RefreshToken)
	}
	if tokens.TokenExpiry != 1234 {
		t.Errorf("TokenExpiry = %d, want 1234", tokens.TokenExpiry)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = 'test-tokens-1'`)
	})
}

func TestIntegration_UpdateTokens(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s, _ := NewUserStore(pool)

	_ = s.UpsertUser(ctx, "test-update-1", "Update User", "old-access", "old-refresh", 1000)

	err := s.UpdateTokens(ctx, "test-update-1", "new-access", "new-refresh", 2000)
	if err != nil {
		t.Fatalf("UpdateTokens: %v", err)
	}

	tokens, err := s.GetTokens(ctx, "test-update-1")
	if err != nil {
		t.Fatalf("GetTokens: %v", err)
	}
	if tokens.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want new-access", tokens.AccessToken)
	}
	if tokens.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want new-refresh", tokens.RefreshToken)
	}
	if tokens.TokenExpiry != 2000 {
		t.Errorf("TokenExpiry = %d, want 2000", tokens.TokenExpiry)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = 'test-update-1'`)
	})
}

func TestIntegration_UpsertGenres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s, _ := NewUserStore(pool)

	_ = s.UpsertUser(ctx, "test-genres-1", "Genre User", "tok", "ref", 9999)

	genres := map[string]float32{"rock": 1.0, "indie": 0.7, "dream pop": 0.3}
	err := s.UpsertGenres(ctx, "test-genres-1", genres)
	if err != nil {
		t.Fatalf("UpsertGenres: %v", err)
	}

	got, err := s.GetGenres(ctx, "test-genres-1")
	if err != nil {
		t.Fatalf("GetGenres: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 genres, got %d", len(got))
	}
	if got["rock"] != 1.0 {
		t.Errorf("rock = %f, want 1.0", got["rock"])
	}
	if got["indie"] != 0.7 {
		t.Errorf("indie = %f, want 0.7", got["indie"])
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM user_genres WHERE user_id = 'test-genres-1'`)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = 'test-genres-1'`)
	})
}

func TestIntegration_GetGenres_Empty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s, _ := NewUserStore(pool)

	_ = s.UpsertUser(ctx, "test-empty-1", "Empty User", "tok", "ref", 9999)

	got, err := s.GetGenres(ctx, "test-empty-1")
	if err != nil {
		t.Fatalf("GetGenres: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 genres, got %d", len(got))
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = 'test-empty-1'`)
	})
}

func TestIntegration_UpsertGenres_Replaces(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s, _ := NewUserStore(pool)

	_ = s.UpsertUser(ctx, "test-replace-1", "Replace User", "tok", "ref", 9999)

	// First upsert.
	_ = s.UpsertGenres(ctx, "test-replace-1", map[string]float32{"rock": 1.0, "pop": 0.5})

	// Second upsert with different genres.
	err := s.UpsertGenres(ctx, "test-replace-1", map[string]float32{"jazz": 0.9, "blues": 0.6})
	if err != nil {
		t.Fatalf("second UpsertGenres: %v", err)
	}

	got, err := s.GetGenres(ctx, "test-replace-1")
	if err != nil {
		t.Fatalf("GetGenres: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 genres (old ones replaced), got %d: %v", len(got), got)
	}
	if _, ok := got["rock"]; ok {
		t.Error("old genre 'rock' should have been replaced")
	}
	if got["jazz"] != 0.9 {
		t.Errorf("jazz = %f, want 0.9", got["jazz"])
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM user_genres WHERE user_id = 'test-replace-1'`)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = 'test-replace-1'`)
	})
}
