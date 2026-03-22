package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pseudo/vibe-seeker/backend/internal/migrations"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Skipf("database unavailable, skipping integration test: %v", err)
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

// --- Venue Store Integration Tests ---

func testVenueStore(t *testing.T) (*VenueStore, *pgxpool.Pool) {
	t.Helper()
	pool := testPool(t)
	s, err := NewVenueStore(pool)
	if err != nil {
		t.Fatalf("NewVenueStore: %v", err)
	}
	return s, pool
}

func TestIntegration_UpsertVenues(t *testing.T) {
	s, pool := testVenueStore(t)
	ctx := context.Background()

	venues := []Venue{
		{ID: "tm_test1", Name: "Test Venue", Latitude: 40.7, Longitude: -74.0, Address: "123 Test St", City: "New York", State: "NY", DataSource: "ticketmaster", TMID: "test1"},
	}
	if err := s.UpsertVenues(ctx, venues); err != nil {
		t.Fatalf("UpsertVenues: %v", err)
	}

	// Upsert again with updated name.
	venues[0].Name = "Updated Venue"
	if err := s.UpsertVenues(ctx, venues); err != nil {
		t.Fatalf("second UpsertVenues: %v", err)
	}

	got, err := s.GetVenues(ctx)
	if err != nil {
		t.Fatalf("GetVenues: %v", err)
	}

	var found bool
	for _, v := range got {
		if v.ID == "tm_test1" {
			found = true
			if v.Name != "Updated Venue" {
				t.Errorf("name = %q, want Updated Venue", v.Name)
			}
		}
	}
	if !found {
		t.Error("test venue not found in GetVenues results")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM venues WHERE id = 'tm_test1'`)
	})
}

func TestIntegration_UpsertShows(t *testing.T) {
	s, pool := testVenueStore(t)
	ctx := context.Background()

	// Create venue first.
	_ = s.UpsertVenues(ctx, []Venue{
		{ID: "tm_showtest", Name: "Show Test Venue", Latitude: 40.7, Longitude: -74.0, DataSource: "ticketmaster", TMID: "showtest"},
	})

	shows := []Show{
		{ID: "tm_show1", Name: "Test Show", VenueID: "tm_showtest", ShowDate: time.Now().Add(24 * time.Hour), Status: "onsale", DataSource: "ticketmaster"},
		{ID: "tm_show2", Name: "Test Show 2", VenueID: "tm_showtest", ShowDate: time.Now().Add(48 * time.Hour), Status: "onsale", DataSource: "ticketmaster"},
	}
	if err := s.UpsertShows(ctx, shows); err != nil {
		t.Fatalf("UpsertShows: %v", err)
	}

	// Verify shows_tracked was updated.
	venues, _ := s.GetVenues(ctx)
	for _, v := range venues {
		if v.ID == "tm_showtest" && v.ShowsTracked != 2 {
			t.Errorf("shows_tracked = %d, want 2", v.ShowsTracked)
		}
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM shows WHERE venue_id = 'tm_showtest'`)
		_, _ = pool.Exec(ctx, `DELETE FROM venues WHERE id = 'tm_showtest'`)
	})
}

func TestIntegration_UpsertArtists(t *testing.T) {
	s, pool := testVenueStore(t)
	ctx := context.Background()

	artists := []Artist{
		{ID: "test-artist", Name: "Test Artist", ImageURL: "https://example.com/img.jpg"},
	}
	if err := s.UpsertArtists(ctx, artists); err != nil {
		t.Fatalf("UpsertArtists: %v", err)
	}

	// Upsert again — should not error.
	if err := s.UpsertArtists(ctx, artists); err != nil {
		t.Fatalf("second UpsertArtists: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM artists WHERE id = 'test-artist'`)
	})
}

func TestIntegration_GetVenueFetchedAt(t *testing.T) {
	s, pool := testVenueStore(t)
	ctx := context.Background()

	// Use a unique data_source to avoid interference from other tests/syncs.
	source := "test_fetchedat"

	got, err := s.GetVenueFetchedAt(ctx, source)
	if err != nil {
		t.Fatalf("GetVenueFetchedAt: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown data source, got %v", got)
	}

	// Add a venue with our test source and check again.
	_ = s.UpsertVenues(ctx, []Venue{
		{ID: "tm_fetch_test", Name: "Fetch Test", Latitude: 40.7, Longitude: -74.0, DataSource: source, TMID: "fetch_test"},
	})

	got, err = s.GetVenueFetchedAt(ctx, source)
	if err != nil {
		t.Fatalf("GetVenueFetchedAt after insert: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil fetched_at after insert")
	}
	if time.Since(*got) > 10*time.Second {
		t.Errorf("fetched_at too old: %v", got)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM venues WHERE id = 'tm_fetch_test'`)
	})
}

func TestIntegration_GetVenues_Empty(t *testing.T) {
	s, _ := testVenueStore(t)
	ctx := context.Background()

	got, err := s.GetVenues(ctx)
	if err != nil {
		t.Fatalf("GetVenues: %v", err)
	}
	// May have venues from other tests, just verify no error.
	_ = got
}
