package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

type mockLastFM struct {
	tags map[string][]lastfm.Tag
	err  error
}

func (m *mockLastFM) FetchArtistTags(_ context.Context, artistName string) ([]lastfm.Tag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags[artistName], nil
}

type mockTagCache struct {
	cached          map[string][]lastfm.Tag
	classifications map[string][]lastfm.Tag
	upserted        map[string][]lastfm.Tag
}

func newMockTagCache() *mockTagCache {
	return &mockTagCache{
		cached:          make(map[string][]lastfm.Tag),
		classifications: make(map[string][]lastfm.Tag),
		upserted:        make(map[string][]lastfm.Tag),
	}
}

func (m *mockTagCache) GetCachedTags(_ context.Context, artistName string) ([]lastfm.Tag, error) {
	if tags, ok := m.cached[artistName]; ok {
		return tags, nil
	}
	return nil, nil
}

func (m *mockTagCache) UpsertArtistTags(_ context.Context, artistName string, tags []lastfm.Tag) error {
	m.upserted[artistName] = tags
	return nil
}

func (m *mockTagCache) UpsertArtistTagsWithSource(_ context.Context, artistName string, tags []lastfm.Tag, _ string) error {
	m.upserted[artistName] = tags
	return nil
}

func (m *mockTagCache) GetClassificationsForArtist(_ context.Context, artistName string) ([]lastfm.Tag, error) {
	return m.classifications[artistName], nil
}

func TestEnrich_CacheHit(t *testing.T) {
	cache := newMockTagCache()
	cache.cached["artist one"] = []lastfm.Tag{{Name: "rock", Count: 100}}

	enricher, _ := NewTagEnricher(&mockLastFM{}, cache)
	result := enricher.Enrich(context.Background(), []string{"artist one"}, 10*time.Millisecond)

	if result.CacheHits != 1 {
		t.Errorf("expected 1 cache hit, got %d", result.CacheHits)
	}
	if len(result.ArtistTags["artist one"]) != 1 {
		t.Error("expected tags for 'artist one'")
	}
}

func TestEnrich_LastFMFetch(t *testing.T) {
	lfm := &mockLastFM{
		tags: map[string][]lastfm.Tag{
			"artist two": {{Name: "indie", Count: 90}},
		},
	}
	cache := newMockTagCache()

	enricher, _ := NewTagEnricher(lfm, cache)
	result := enricher.Enrich(context.Background(), []string{"artist two"}, 10*time.Millisecond)

	if result.CacheHits != 0 {
		t.Errorf("expected 0 cache hits, got %d", result.CacheHits)
	}
	if len(result.ArtistTags["artist two"]) != 1 {
		t.Error("expected tags for 'artist two'")
	}
	// Should have been cached.
	if len(cache.upserted["artist two"]) != 1 {
		t.Error("expected tags to be cached")
	}
}

func TestEnrich_TMFallback(t *testing.T) {
	lfm := &mockLastFM{tags: map[string][]lastfm.Tag{}} // Last.fm returns empty
	cache := newMockTagCache()
	cache.classifications["artist three"] = []lastfm.Tag{{Name: "pop", Count: 80}}

	enricher, _ := NewTagEnricher(lfm, cache)
	result := enricher.Enrich(context.Background(), []string{"artist three"}, 10*time.Millisecond)

	if len(result.ArtistTags["artist three"]) != 1 {
		t.Error("expected TM fallback tags for 'artist three'")
	}
	if result.ArtistTags["artist three"][0].Name != "pop" {
		t.Errorf("expected 'pop' tag, got %q", result.ArtistTags["artist three"][0].Name)
	}
}

func TestEnrich_LastFMError(t *testing.T) {
	lfm := &mockLastFM{err: errors.New("api error")}
	cache := newMockTagCache()

	enricher, _ := NewTagEnricher(lfm, cache)
	result := enricher.Enrich(context.Background(), []string{"artist four"}, 10*time.Millisecond)

	if len(result.ArtistTags) != 0 {
		t.Errorf("expected no tags on error, got %d", len(result.ArtistTags))
	}
}

func TestEnrich_ContextCanceled(t *testing.T) {
	lfm := &mockLastFM{tags: map[string][]lastfm.Tag{
		"a": {{Name: "rock", Count: 100}},
		"b": {{Name: "jazz", Count: 90}},
	}}
	cache := newMockTagCache()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	enricher, _ := NewTagEnricher(lfm, cache)
	result := enricher.Enrich(ctx, []string{"a", "b"}, 10*time.Millisecond)

	// With a canceled context, should not fetch any tags.
	if len(result.ArtistTags) != 0 {
		t.Errorf("expected no tags with canceled context, got %d", len(result.ArtistTags))
	}
}

func TestEnrich_MixedCacheAndFetch(t *testing.T) {
	cache := newMockTagCache()
	cache.cached["cached-artist"] = []lastfm.Tag{{Name: "rock", Count: 100}}

	lfm := &mockLastFM{
		tags: map[string][]lastfm.Tag{
			"new-artist": {{Name: "electronic", Count: 85}},
		},
	}

	enricher, _ := NewTagEnricher(lfm, cache)
	result := enricher.Enrich(context.Background(), []string{"cached-artist", "new-artist"}, 10*time.Millisecond)

	if result.CacheHits != 1 {
		t.Errorf("expected 1 cache hit, got %d", result.CacheHits)
	}
	if len(result.ArtistTags) != 2 {
		t.Errorf("expected 2 artist tags, got %d", len(result.ArtistTags))
	}
}

func TestNewTagEnricher_NilDeps(t *testing.T) {
	cache := newMockTagCache()
	lfm := &mockLastFM{}

	tests := []struct {
		name  string
		lfm   LastFMClient
		cache TagCache
	}{
		{"nil lastfm", nil, cache},
		{"nil cache", lfm, nil},
	}

	for _, tt := range tests {
		_, err := NewTagEnricher(tt.lfm, tt.cache)
		if err == nil {
			t.Errorf("%s: expected error", tt.name)
		}
	}
}
