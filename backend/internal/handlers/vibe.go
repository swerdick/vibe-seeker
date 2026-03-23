package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/ratelimit"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

// TokenStore reads and writes Spotify tokens.
type TokenStore interface {
	GetTokens(ctx context.Context, userID string) (*store.UserTokens, error)
	UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, tokenExpiry int) error
}

// VibeStore reads and writes user vibe profiles.
type VibeStore interface {
	UpsertVibes(ctx context.Context, userID string, vibes map[string]float32) error
	GetVibes(ctx context.Context, userID string) (map[string]float32, error)
}

// TagCache provides read-through caching for Last.fm artist tags.
type TagCache interface {
	GetCachedTags(ctx context.Context, artistName string) ([]lastfm.Tag, error)
	UpsertArtistTags(ctx context.Context, artistName string, tags []lastfm.Tag) error
}

// VibeHandler orchestrates vibe profile syncing via Spotify (top artists)
// and Last.fm (genre/tag enrichment).
type VibeHandler struct {
	Spotify  *spotify.Client
	LastFM   *lastfm.Client
	Tokens   TokenStore
	Vibes    VibeStore
	TagCache TagCache
}

func NewVibeHandler(sp *spotify.Client, lfm *lastfm.Client, tokens TokenStore, vibes VibeStore, tagCache TagCache) (*VibeHandler, error) {
	if sp == nil {
		return nil, errors.New("vibe: nil spotify client")
	}
	if lfm == nil {
		return nil, errors.New("vibe: nil lastfm client")
	}
	if tokens == nil {
		return nil, errors.New("vibe: nil token store")
	}
	if vibes == nil {
		return nil, errors.New("vibe: nil vibe store")
	}
	if tagCache == nil {
		return nil, errors.New("vibe: nil tag cache")
	}
	return &VibeHandler{
		Spotify:  sp,
		LastFM:   lfm,
		Tokens:   tokens,
		Vibes:    vibes,
		TagCache: tagCache,
	}, nil
}

// SyncVibe fetches the user's top artists from Spotify, enriches them with
// Last.fm tags, computes weighted tag scores, and persists the result.
func (h *VibeHandler) SyncVibe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()

	accessToken, err := h.ensureValidToken(ctx, claims.SpotifyID)
	if err != nil {
		httpError(w, r, http.StatusBadGateway, "failed to authenticate with spotify",
			"failed to get valid token", "user", claims.SpotifyID, "error", err)
		return
	}

	mediumResp, err := h.Spotify.FetchTopArtists(ctx, accessToken, "medium_term", 50)
	if err != nil {
		httpError(w, r, http.StatusBadGateway, "failed to fetch top artists",
			"failed to fetch medium-term top artists", "error", err)
		return
	}

	shortResp, err := h.Spotify.FetchTopArtists(ctx, accessToken, "short_term", 50)
	if err != nil {
		httpError(w, r, http.StatusBadGateway, "failed to fetch top artists",
			"failed to fetch short-term top artists", "error", err)
		return
	}

	// Build rankings and collect unique artist names.
	var rankings []lastfm.ArtistRanking
	seen := make(map[string]bool)

	for i, a := range mediumResp.Items {
		rankings = append(rankings, lastfm.ArtistRanking{
			Name: a.Name, Position: i, RangeMultiplier: 1.0,
		})
		seen[strings.ToLower(a.Name)] = true
	}
	for i, a := range shortResp.Items {
		rankings = append(rankings, lastfm.ArtistRanking{
			Name: a.Name, Position: i, RangeMultiplier: 0.5,
		})
		seen[strings.ToLower(a.Name)] = true
	}

	// Fetch Last.fm tags for each unique artist, using the DB cache first.
	tagCtx, tagCancel := context.WithTimeout(context.WithoutCancel(ctx), configuration.VibeSyncTimeout)
	defer tagCancel()

	limiter := ratelimit.NewLimiter(configuration.LastFMRateLimit)
	defer limiter.Stop()

	artistTags := make(map[string][]lastfm.Tag)
	cacheHits := 0
	for name := range seen {
		if tagCtx.Err() != nil {
			observability.Logger(ctx).Error("lastfm tag fetch timed out", "fetched", len(artistTags), "total", len(seen))
			break
		}

		// Check cache first.
		cached, err := h.TagCache.GetCachedTags(tagCtx, name)
		if err != nil {
			observability.Logger(ctx).Error("cache lookup failed", "artist", name, "error", err)
		}
		if cached != nil {
			artistTags[name] = cached
			cacheHits++
			continue
		}

		// Cache miss — fetch from Last.fm (rate-limited).
		if err := limiter.Wait(tagCtx); err != nil {
			observability.Logger(ctx).Error("lastfm tag fetch timed out during wait", "error", err)
			break
		}
		tags, err := h.LastFM.FetchArtistTags(tagCtx, name)
		if err != nil {
			observability.Logger(ctx).Error("failed to fetch lastfm tags", "artist", name, "error", err)
			continue
		}
		artistTags[name] = tags

		// Persist to cache.
		if cacheErr := h.TagCache.UpsertArtistTags(tagCtx, name, tags); cacheErr != nil {
			observability.Logger(ctx).Error("failed to cache lastfm tags", "artist", name, "error", cacheErr)
		}
	}
	observability.Logger(ctx).Info("tag fetch complete", "total", len(seen), "cache_hits", cacheHits, "api_calls", len(seen)-cacheHits)

	weights := lastfm.ComputeTagWeights(artistTags, rankings)

	// Use a fresh context for the DB write — the data is already in memory,
	// so we don't want the Last.fm timeout to prevent saving results.
	dbCtx := context.WithoutCancel(ctx)
	if err := h.Vibes.UpsertVibes(dbCtx, claims.SpotifyID, weights); err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to persist genre weights", "error", err)
		return
	}

	observability.Logger(ctx).Info("vibe sync complete", "user", claims.SpotifyID, "tags", len(weights))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":      true,
		"genre_count": len(weights),
	})
}

// GetVibe returns the authenticated user's genre weights.
func (h *VibeHandler) GetVibe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vibes, err := h.Vibes.GetVibes(r.Context(), claims.SpotifyID)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to read genre weights", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"genres":      vibes,
		"genre_count": len(vibes),
	})
}

// ensureValidToken returns a valid access token, refreshing it if expired.
func (h *VibeHandler) ensureValidToken(ctx context.Context, userID string) (string, error) {
	tokens, err := h.Tokens.GetTokens(ctx, userID)
	if err != nil {
		return "", err
	}

	if int64(tokens.TokenExpiry) <= time.Now().Unix()+configuration.TokenRefreshThreshold {
		refreshed, err := h.Spotify.RefreshToken(ctx, tokens.RefreshToken)
		if err != nil {
			return "", err
		}

		newExpiry := int(time.Now().Unix()) + refreshed.ExpiresIn
		if err := h.Tokens.UpdateTokens(ctx, userID, refreshed.AccessToken, refreshed.RefreshToken, newExpiry); err != nil {
			return "", err
		}

		return refreshed.AccessToken, nil
	}

	return tokens.AccessToken, nil
}
