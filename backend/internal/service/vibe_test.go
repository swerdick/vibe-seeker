package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

type mockSpotifyVibe struct {
	topArtists map[string]*spotify.TopArtistsResponse
	refreshResp *spotify.TokenResponse
	fetchErr    error
	refreshErr  error
}

func (m *mockSpotifyVibe) FetchTopArtists(_ context.Context, _, timeRange string, _ int) (*spotify.TopArtistsResponse, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.topArtists[timeRange], nil
}

func (m *mockSpotifyVibe) RefreshToken(_ context.Context, _ string) (*spotify.TokenResponse, error) {
	return m.refreshResp, m.refreshErr
}

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

func newTestVibeService(t *testing.T) (*VibeService, *mockVibeStore) {
	t.Helper()

	sp := &mockSpotifyVibe{
		topArtists: map[string]*spotify.TopArtistsResponse{
			"medium_term": {Items: []spotify.Artist{
				{ID: "a1", Name: "Artist One"},
				{ID: "a2", Name: "Artist Two"},
			}},
			"short_term": {Items: []spotify.Artist{
				{ID: "a3", Name: "Artist Three"},
			}},
		},
	}

	cache := newMockTagCache()
	cache.cached["artist one"] = []lastfm.Tag{{Name: "rock", Count: 100}}
	cache.cached["artist two"] = []lastfm.Tag{{Name: "indie", Count: 80}}
	cache.cached["artist three"] = []lastfm.Tag{{Name: "jazz", Count: 70}}

	enricher := NewTagEnricher(&mockLastFM{}, cache)
	vibeStore := &mockVibeStore{}

	svc, err := NewVibeService(sp,
		&mockTokenStore{tokens: &store.UserTokens{
			AccessToken: "valid-token", RefreshToken: "refresh",
			TokenExpiry: int(time.Now().Unix()) + 3600,
		}},
		vibeStore,
		enricher,
	)
	if err != nil {
		t.Fatalf("NewVibeService: %v", err)
	}
	return svc, vibeStore
}

func TestSyncVibe_Success(t *testing.T) {
	svc, vibeStore := newTestVibeService(t)

	result, err := svc.SyncVibe(context.Background(), "user1")
	if err != nil {
		t.Fatalf("SyncVibe: %v", err)
	}

	if result.VibeCount == 0 {
		t.Error("expected non-zero vibe count")
	}
	if !vibeStore.upsertCalled {
		t.Error("expected UpsertVibes to be called")
	}
	if _, ok := vibeStore.upsertVibes["rock"]; !ok {
		t.Error("expected 'rock' in computed vibes")
	}
}

func TestSyncVibe_TokenRefresh(t *testing.T) {
	sp := &mockSpotifyVibe{
		topArtists: map[string]*spotify.TopArtistsResponse{
			"medium_term": {Items: []spotify.Artist{{ID: "a1", Name: "A"}}},
			"short_term":  {Items: []spotify.Artist{}},
		},
		refreshResp: &spotify.TokenResponse{
			AccessToken: "new-token", RefreshToken: "new-refresh", ExpiresIn: 3600,
		},
	}

	tokenStore := &mockTokenStore{tokens: &store.UserTokens{
		AccessToken: "expired", RefreshToken: "old-refresh",
		TokenExpiry: int(time.Now().Unix()) - 100,
	}}

	cache := newMockTagCache()
	cache.cached["a"] = []lastfm.Tag{{Name: "rock", Count: 100}}

	svc, _ := NewVibeService(sp, tokenStore, &mockVibeStore{}, NewTagEnricher(&mockLastFM{}, cache))

	_, err := svc.SyncVibe(context.Background(), "user1")
	if err != nil {
		t.Fatalf("SyncVibe: %v", err)
	}

	if !tokenStore.updateCalled {
		t.Error("expected token refresh to be triggered")
	}
}

func TestSyncVibe_SpotifyError(t *testing.T) {
	sp := &mockSpotifyVibe{fetchErr: errors.New("spotify down")}
	cache := newMockTagCache()

	svc, _ := NewVibeService(sp,
		&mockTokenStore{tokens: &store.UserTokens{
			AccessToken: "valid", RefreshToken: "refresh",
			TokenExpiry: int(time.Now().Unix()) + 3600,
		}},
		&mockVibeStore{},
		NewTagEnricher(&mockLastFM{}, cache),
	)

	_, err := svc.SyncVibe(context.Background(), "user1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetVibe_Success(t *testing.T) {
	svc, _ := newTestVibeService(t)
	// Override the vibe store's get result directly through the service
	// by creating a new service with specific store state.
	sp := &mockSpotifyVibe{}
	vibeStore := &mockVibeStore{getVibes: map[string]float32{"rock": 1.0, "indie": 0.7}}
	cache := newMockTagCache()
	svc2, _ := NewVibeService(sp,
		&mockTokenStore{tokens: &store.UserTokens{
			AccessToken: "t", RefreshToken: "r",
			TokenExpiry: int(time.Now().Unix()) + 3600,
		}},
		vibeStore,
		NewTagEnricher(&mockLastFM{}, cache),
	)
	_ = svc // silence unused

	vibes, err := svc2.GetVibe(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetVibe: %v", err)
	}
	if vibes["rock"] != 1.0 {
		t.Errorf("rock = %f, want 1.0", vibes["rock"])
	}
}

func TestGetVibe_Error(t *testing.T) {
	sp := &mockSpotifyVibe{}
	cache := newMockTagCache()
	svc, _ := NewVibeService(sp,
		&mockTokenStore{tokens: &store.UserTokens{
			AccessToken: "t", RefreshToken: "r",
			TokenExpiry: int(time.Now().Unix()) + 3600,
		}},
		&mockVibeStore{getErr: errors.New("db error")},
		NewTagEnricher(&mockLastFM{}, cache),
	)

	_, err := svc.GetVibe(context.Background(), "user1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewVibeService_NilDeps(t *testing.T) {
	cache := newMockTagCache()
	enricher := NewTagEnricher(&mockLastFM{}, cache)
	tokens := &mockTokenStore{tokens: &store.UserTokens{}}
	vibes := &mockVibeStore{}
	sp := &mockSpotifyVibe{}

	tests := []struct {
		name string
		sp   SpotifyVibeClient
		tok  TokenStore
		vib  VibeStore
		enr  *TagEnricher
	}{
		{"nil spotify", nil, tokens, vibes, enricher},
		{"nil tokens", sp, nil, vibes, enricher},
		{"nil vibes", sp, tokens, nil, enricher},
		{"nil enricher", sp, tokens, vibes, nil},
	}

	for _, tt := range tests {
		_, err := NewVibeService(tt.sp, tt.tok, tt.vib, tt.enr)
		if err == nil {
			t.Errorf("%s: expected error", tt.name)
		}
	}
}
