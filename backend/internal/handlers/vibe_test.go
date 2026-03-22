package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

type mockTokenReader struct {
	tokens *store.UserTokens
	err    error
}

func (m *mockTokenReader) GetTokens(_ context.Context, _ string) (*store.UserTokens, error) {
	return m.tokens, m.err
}

type mockTokenWriter struct {
	called bool
	err    error
}

func (m *mockTokenWriter) UpdateTokens(_ context.Context, _, _, _ string, _ int) error {
	m.called = true
	return m.err
}

type mockGenreStore struct {
	upsertCalled bool
	upsertGenres map[string]float32
	upsertErr    error
	getGenres    map[string]float32
	getErr       error
}

func (m *mockGenreStore) UpsertGenres(_ context.Context, _ string, genres map[string]float32) error {
	m.upsertCalled = true
	m.upsertGenres = genres
	return m.upsertErr
}

func (m *mockGenreStore) GetGenres(_ context.Context, _ string) (map[string]float32, error) {
	return m.getGenres, m.getErr
}

func newMockServers(t *testing.T) (*spotify.Client, *lastfm.Client, func()) {
	t.Helper()

	spotifyMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/me/top/artists":
			timeRange := r.URL.Query().Get("time_range")
			var artists []spotify.Artist
			if timeRange == "medium_term" {
				artists = []spotify.Artist{
					{ID: "a1", Name: "Artist One"},
					{ID: "a2", Name: "Artist Two"},
				}
			} else {
				artists = []spotify.Artist{
					{ID: "a3", Name: "Artist Three"},
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": artists})
		case "/api/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "refreshed-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))

	lastfmMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"toptags": map[string]interface{}{
				"tag": []map[string]interface{}{
					{"name": "rock", "count": 100},
					{"name": "indie", "count": 80},
				},
			},
		})
	}))

	sp := spotify.NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	sp.TopArtistsURL = spotifyMock.URL + "/v1/me/top/artists"
	sp.TokenURL = spotifyMock.URL + "/api/token"
	sp.HTTPClient = spotifyMock.Client()

	lfm := lastfm.NewClient("test-key")
	lfm.BaseURL = lastfmMock.URL + "/"
	lfm.HTTPClient = lastfmMock.Client()

	cleanup := func() {
		spotifyMock.Close()
		lastfmMock.Close()
	}

	return sp, lfm, cleanup
}

func TestSyncVibe_Success(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	genreStore := &mockGenreStore{}
	h, err := NewVibeHandler(sp, lfm,
		&mockTokenReader{tokens: &store.UserTokens{
			AccessToken: "valid-token", RefreshToken: "refresh", TokenExpiry: int(time.Now().Unix()) + 3600,
		}},
		&mockTokenWriter{},
		genreStore,
	)
	if err != nil {
		t.Fatalf("NewVibeHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vibe/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVibe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !genreStore.upsertCalled {
		t.Fatal("expected UpsertGenres to be called")
	}

	if _, ok := genreStore.upsertGenres["rock"]; !ok {
		t.Error("expected 'rock' in genre weights (from Last.fm tags)")
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["synced"] != true {
		t.Error("expected synced=true")
	}
}

func TestSyncVibe_Unauthorized(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	h, _ := NewVibeHandler(sp, lfm,
		&mockTokenReader{tokens: &store.UserTokens{}},
		&mockTokenWriter{},
		&mockGenreStore{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/vibe/sync", nil)
	rec := httptest.NewRecorder()

	h.SyncVibe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSyncVibe_TokenRefresh(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	tokenWriter := &mockTokenWriter{}
	h, _ := NewVibeHandler(sp, lfm,
		&mockTokenReader{tokens: &store.UserTokens{
			AccessToken: "expired-token", RefreshToken: "refresh", TokenExpiry: int(time.Now().Unix()) - 100,
		}},
		tokenWriter,
		&mockGenreStore{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/vibe/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVibe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !tokenWriter.called {
		t.Error("expected UpdateTokens to be called for expired token")
	}
}

func TestGetVibe_Success(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	genreStore := &mockGenreStore{
		getGenres: map[string]float32{"rock": 1.0, "indie": 0.7},
	}
	h, _ := NewVibeHandler(sp, lfm,
		&mockTokenReader{tokens: &store.UserTokens{}},
		&mockTokenWriter{},
		genreStore,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/vibe", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.GetVibe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	genres, ok := body["genres"].(map[string]interface{})
	if !ok {
		t.Fatal("expected genres map in response")
	}
	if genres["rock"] == nil {
		t.Error("expected 'rock' in genres")
	}
}

func TestGetVibe_Unauthorized(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	h, _ := NewVibeHandler(sp, lfm,
		&mockTokenReader{tokens: &store.UserTokens{}},
		&mockTokenWriter{},
		&mockGenreStore{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/vibe", nil)
	rec := httptest.NewRecorder()

	h.GetVibe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// addTestClaims creates a JWT and injects claims into the request context
// via the auth middleware, mimicking an authenticated request.
func addTestClaims(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	token, err := auth.CreateToken("jwt-secret", "spotify123", "Test User")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	claims, err := auth.ParseToken("jwt-secret", token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	ctx := middleware.ContextWithClaims(req.Context(), claims)
	return req.WithContext(ctx)
}
