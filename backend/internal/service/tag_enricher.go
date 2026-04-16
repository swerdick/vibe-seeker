package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/ratelimit"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

// LastFMClient fetches artist tags from the Last.fm API.
type LastFMClient interface {
	FetchArtistTags(ctx context.Context, artistName string) ([]lastfm.Tag, error)
}

// TagCache provides read-through caching for artist tags.
type TagCache interface {
	GetCachedTags(ctx context.Context, artistName string) ([]lastfm.Tag, error)
	UpsertArtistTags(ctx context.Context, artistName string, tags []lastfm.Tag) error
	UpsertArtistTagsWithSource(ctx context.Context, artistName string, tags []lastfm.Tag, source string) error
	GetClassificationsForArtist(ctx context.Context, artistName string) ([]lastfm.Tag, error)
	GetStaleArtistNames(ctx context.Context, olderThan time.Duration) ([]string, error)
}

// TagEnricher resolves artist tags using a cache-first strategy:
// DB cache → Last.fm API → Ticketmaster classifications fallback.
type TagEnricher struct {
	lastFM   LastFMClient
	tagCache TagCache
}

// EnrichResult holds the output of an artist tag enrichment pass.
type EnrichResult struct {
	ArtistTags map[string][]lastfm.Tag
	CacheHits  int
}

// NewTagEnricher creates a TagEnricher with the given Last.fm client and cache.
func NewTagEnricher(lfm LastFMClient, cache TagCache) (*TagEnricher, error) {
	if lfm == nil {
		return nil, errors.New("tag enricher: nil lastfm client")
	}
	if cache == nil {
		return nil, errors.New("tag enricher: nil tag cache")
	}
	return &TagEnricher{lastFM: lfm, tagCache: cache}, nil
}

// Enrich fetches tags for each artist name using the cache-first strategy.
// It rate-limits Last.fm API calls at the given interval.
func (e *TagEnricher) Enrich(ctx context.Context, artistNames []string, rateLimit time.Duration) EnrichResult {
	log := observability.Logger(ctx)

	limiter := ratelimit.NewLimiter(rateLimit)
	defer limiter.Stop()

	result := EnrichResult{ArtistTags: make(map[string][]lastfm.Tag)}

	for _, name := range artistNames {
		if ctx.Err() != nil {
			log.Error("tag enrichment timed out", "fetched", len(result.ArtistTags), "total", len(artistNames))
			break
		}

		tags, fromCache := e.enrichOne(ctx, log, limiter, name)
		if tags != nil {
			result.ArtistTags[name] = tags
			if fromCache {
				result.CacheHits++
			}
		}
	}

	log.Info("tag enrichment complete", "total", len(artistNames), "cache_hits", result.CacheHits, "api_calls", len(artistNames)-result.CacheHits)
	return result
}

// EnrichStale re-enriches all artists whose cached tags are older than the
// configured ArtistTagCacheTTL. Used by the background tag enrichment job.
func (e *TagEnricher) EnrichStale(ctx context.Context) error {
	log := observability.Logger(ctx)

	names, err := e.tagCache.GetStaleArtistNames(ctx, configuration.ArtistTagCacheTTL)
	if err != nil {
		return fmt.Errorf("listing stale artists: %w", err)
	}

	if len(names) == 0 {
		log.Info("no stale artist tags to refresh")
		return nil
	}

	log.Info("refreshing stale artist tags", "count", len(names))
	e.Enrich(ctx, names, configuration.LastFMRateLimit)
	return nil
}

// enrichOne resolves tags for a single artist: cache → Last.fm → TM fallback.
// Returns the tags and whether they came from cache.
func (e *TagEnricher) enrichOne(ctx context.Context, log *slog.Logger, limiter *ratelimit.Limiter, name string) ([]lastfm.Tag, bool) {
	// 1. Check cache.
	cached, err := e.tagCache.GetCachedTags(ctx, name)
	if err != nil {
		log.Error("cache lookup failed", "artist", name, "error", err)
	}
	if cached != nil {
		return cached, true
	}

	// 2. Cache miss — fetch from Last.fm (rate-limited).
	if err := limiter.Wait(ctx); err != nil {
		log.Error("tag fetch timed out during rate-limit wait", "error", err)
		return nil, false
	}
	tags, err := e.lastFM.FetchArtistTags(ctx, name)
	if err != nil {
		log.Error("failed to fetch lastfm tags", "artist", name, "error", err)
		return nil, false
	}

	if len(tags) > 0 {
		if cacheErr := e.tagCache.UpsertArtistTags(ctx, name, tags); cacheErr != nil {
			log.Error("failed to cache lastfm tags", "artist", name, "error", cacheErr)
		}
		return tags, false
	}

	// 3. Last.fm miss — fall back to Ticketmaster classifications.
	tmTags, tmErr := e.tagCache.GetClassificationsForArtist(ctx, name)
	if tmErr != nil {
		log.Error("failed to get TM classifications", "artist", name, "error", tmErr)
		return nil, false
	}
	if len(tmTags) > 0 {
		log.Info("using ticketmaster classifications as fallback", "artist", name, "tags", len(tmTags))
		if cacheErr := e.tagCache.UpsertArtistTagsWithSource(ctx, name, tmTags, store.TagSourceTicketmaster); cacheErr != nil {
			log.Error("failed to cache TM tags", "artist", name, "error", cacheErr)
		}
		return tmTags, false
	}

	log.Info("no tag data available for artist", "artist", name)
	return nil, false
}
