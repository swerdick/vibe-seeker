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

type mockTokenStore struct {
	tokens       *store.UserTokens
	err          error
	updateCalled bool
}

func (m *mockTokenStore) GetTokens(_ context.Context, _ string) (*store.UserTokens, error) {
	return m.tokens, m.err
}

func (m *mockTokenStore) UpdateTokens(_ context.Context, _, _, _ string, _ int) error {
	m.updateCalled = true
	return m.err
}

type mockVibeStore struct {
	upsertCalled bool
	upsertVibes  map[string]float32
	upsertErr    error
	getVibes     map[string]float32
	getErr       error
}

func (m *mockVibeStore) UpsertVibes(_ context.Context, _ string, vibes map[string]float32) error {
	m.upsertCalled = true
	m.upsertVibes = vibes
	return m.upsertErr
}

func (m *mockVibeStore) GetVibes(_ context.Context, _ string) (map[string]float32, error) {
	return m.getVibes, m.getErr
}

type mockTagCache struct {
	tags map[string][]lastfm.Tag
}

func (m *mockTagCache) GetCachedTags(_ context.Context, artistName string) ([]lastfm.Tag, error) {
	if tags, ok := m.tags[artistName]; ok {
		return tags, nil
	}
	return nil, nil
}

func (m *mockTagCache) UpsertArtistTags(_ context.Context, artistName string, tags []lastfm.Tag) error {
	if m.tags == nil {
		m.tags = make(map[string][]lastfm.Tag)
	}
	m.tags[artistName] = tags
	return nil
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

	vibeStore := &mockVibeStore{}
	h, err := NewVibeHandler(sp, lfm,
		&mockTokenStore{tokens: &store.UserTokens{
			AccessToken: "valid-token", RefreshToken: "refresh", TokenExpiry: int(time.Now().Unix()) + 3600,
		}},
		vibeStore,
		&mockTagCache{},
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

	if !vibeStore.upsertCalled {
		t.Fatal("expected UpsertVibes to be called")
	}

	if _, ok := vibeStore.upsertVibes["rock"]; !ok {
		t.Error("expected 'rock' in vibe weights (from Last.fm tags)")
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
		&mockTokenStore{tokens: &store.UserTokens{}},
		&mockVibeStore{},
		&mockTagCache{},
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

	tokenStore := &mockTokenStore{tokens: &store.UserTokens{
		AccessToken: "expired-token", RefreshToken: "refresh", TokenExpiry: int(time.Now().Unix()) - 100,
	}}
	h, _ := NewVibeHandler(sp, lfm,
		tokenStore,
		&mockVibeStore{},
		&mockTagCache{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/vibe/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVibe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !tokenStore.updateCalled {
		t.Error("expected UpdateTokens to be called for expired token")
	}
}

func TestGetVibe_Success(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	vibeStore := &mockVibeStore{
		getVibes: map[string]float32{"rock": 1.0, "indie": 0.7},
	}
	h, _ := NewVibeHandler(sp, lfm,
		&mockTokenStore{tokens: &store.UserTokens{}},
		vibeStore,
		&mockTagCache{},
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

	vibes, ok := body["genres"].(map[string]interface{})
	if !ok {
		t.Fatal("expected genres map in response")
	}
	if vibes["rock"] == nil {
		t.Error("expected 'rock' in vibes")
	}
}

func TestGetVibe_Unauthorized(t *testing.T) {
	sp, lfm, cleanup := newMockServers(t)
	defer cleanup()

	h, _ := NewVibeHandler(sp, lfm,
		&mockTokenStore{tokens: &store.UserTokens{}},
		&mockVibeStore{},
		&mockTagCache{},
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
