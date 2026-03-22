package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

// TokenReader retrieves stored Spotify tokens for a user.
type TokenReader interface {
	GetTokens(ctx context.Context, userID string) (*store.UserTokens, error)
}

// TokenWriter updates stored Spotify tokens after a refresh.
type TokenWriter interface {
	UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, tokenExpiry int) error
}

// GenreWriter persists a user's genre weights.
type GenreWriter interface {
	UpsertGenres(ctx context.Context, userID string, genres map[string]float32) error
}

// GenreReader retrieves a user's genre weights.
type GenreReader interface {
	GetGenres(ctx context.Context, userID string) (map[string]float32, error)
}

// TasteHandler orchestrates taste profile syncing via Spotify (top artists)
// and Last.fm (genre/tag enrichment).
type TasteHandler struct {
	Spotify      *spotify.Client
	LastFM       *lastfm.Client
	Tokens       TokenReader
	TokenUpdater TokenWriter
	Genres       GenreWriter
	GenreReader  GenreReader
}

func NewTasteHandler(sp *spotify.Client, lfm *lastfm.Client, tokens TokenReader, tokenUpdater TokenWriter, genres interface {
	GenreWriter
	GenreReader
}) (*TasteHandler, error) {
	if sp == nil {
		return nil, errors.New("taste: nil spotify client")
	}
	if lfm == nil {
		return nil, errors.New("taste: nil lastfm client")
	}
	if tokens == nil {
		return nil, errors.New("taste: nil token reader")
	}
	if tokenUpdater == nil {
		return nil, errors.New("taste: nil token writer")
	}
	if genres == nil {
		return nil, errors.New("taste: nil genre store")
	}
	return &TasteHandler{
		Spotify:      sp,
		LastFM:       lfm,
		Tokens:       tokens,
		TokenUpdater: tokenUpdater,
		Genres:       genres,
		GenreReader:  genres,
	}, nil
}

// SyncTaste fetches the user's top artists from Spotify, enriches them with
// Last.fm tags, computes weighted tag scores, and persists the result.
func (h *TasteHandler) SyncTaste(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()

	accessToken, err := h.ensureValidToken(ctx, claims.SpotifyID)
	if err != nil {
		observability.Logger(ctx).Error("failed to get valid token", "user", claims.SpotifyID, "error", err)
		http.Error(w, "failed to authenticate with spotify", http.StatusBadGateway)
		return
	}

	mediumResp, err := h.Spotify.FetchTopArtists(ctx, accessToken, "medium_term", 50)
	if err != nil {
		observability.Logger(ctx).Error("failed to fetch medium-term top artists", "error", err)
		http.Error(w, "failed to fetch top artists", http.StatusBadGateway)
		return
	}

	shortResp, err := h.Spotify.FetchTopArtists(ctx, accessToken, "short_term", 50)
	if err != nil {
		observability.Logger(ctx).Error("failed to fetch short-term top artists", "error", err)
		http.Error(w, "failed to fetch top artists", http.StatusBadGateway)
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

	// Fetch Last.fm tags for each unique artist.
	// Use a standalone timeout so the sync isn't killed by a browser disconnect
	// but also can't run unbounded.
	// Rate-limit to 5 req/sec (200ms interval) to stay within Last.fm's limits.
	tagCtx, tagCancel := context.WithTimeout(context.WithoutCancel(ctx), 1*time.Minute)
	defer tagCancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	artistTags := make(map[string][]lastfm.Tag)
	for name := range seen {
		if tagCtx.Err() != nil {
			observability.Logger(ctx).Error("lastfm tag fetch timed out", "fetched", len(artistTags), "total", len(seen))
			break
		}
		<-ticker.C
		tags, err := h.LastFM.FetchArtistTags(tagCtx, name)
		if err != nil {
			observability.Logger(ctx).Error("failed to fetch lastfm tags", "artist", name, "error", err)
			continue // skip this artist, don't fail the whole sync
		}
		artistTags[name] = tags
	}

	weights := lastfm.ComputeTagWeights(artistTags, rankings)

	// Use a fresh context for the DB write — the data is already in memory,
	// so we don't want the Last.fm timeout to prevent saving results.
	dbCtx := context.WithoutCancel(ctx)
	if err := h.Genres.UpsertGenres(dbCtx, claims.SpotifyID, weights); err != nil {
		observability.Logger(ctx).Error("failed to persist genre weights", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	observability.Logger(ctx).Info("taste sync complete", "user", claims.SpotifyID, "tags", len(weights))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":      true,
		"genre_count": len(weights),
	})
}

// GetTaste returns the authenticated user's genre weights.
func (h *TasteHandler) GetTaste(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	genres, err := h.GenreReader.GetGenres(r.Context(), claims.SpotifyID)
	if err != nil {
		observability.Logger(r.Context()).Error("failed to read genre weights", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"genres":      genres,
		"genre_count": len(genres),
	})
}

// ensureValidToken returns a valid access token, refreshing it if expired.
func (h *TasteHandler) ensureValidToken(ctx context.Context, userID string) (string, error) {
	tokens, err := h.Tokens.GetTokens(ctx, userID)
	if err != nil {
		return "", err
	}

	// If the token expires within 60 seconds, refresh it.
	if int64(tokens.TokenExpiry) <= time.Now().Unix()+60 {
		refreshed, err := h.Spotify.RefreshToken(ctx, tokens.RefreshToken)
		if err != nil {
			return "", err
		}

		newExpiry := int(time.Now().Unix()) + refreshed.ExpiresIn
		if err := h.TokenUpdater.UpdateTokens(ctx, userID, refreshed.AccessToken, refreshed.RefreshToken, newExpiry); err != nil {
			return "", err
		}

		return refreshed.AccessToken, nil
	}

	return tokens.AccessToken, nil
}
