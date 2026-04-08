package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/vibes"
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

// SpotifyVibeClient provides the Spotify methods needed for vibe syncing.
type SpotifyVibeClient interface {
	FetchTopArtists(ctx context.Context, accessToken, timeRange string, limit int) (*spotify.TopArtistsResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*spotify.TokenResponse, error)
}

// VibeSyncResult contains the outcome of a vibe sync operation.
type VibeSyncResult struct {
	VibeCount int
}

// VibeService orchestrates user vibe profile syncing: Spotify top artists →
// tag enrichment → vibe computation → persistence.
type VibeService struct {
	spotify     SpotifyVibeClient
	tokens      TokenStore
	vibes       VibeStore
	tagEnricher *TagEnricher
}

// NewVibeService creates a VibeService.
func NewVibeService(sp SpotifyVibeClient, tokens TokenStore, vibeStore VibeStore, enricher *TagEnricher) (*VibeService, error) {
	if sp == nil {
		return nil, errors.New("vibe service: nil spotify client")
	}
	if tokens == nil {
		return nil, errors.New("vibe service: nil token store")
	}
	if vibeStore == nil {
		return nil, errors.New("vibe service: nil vibe store")
	}
	if enricher == nil {
		return nil, errors.New("vibe service: nil tag enricher")
	}
	return &VibeService{
		spotify:     sp,
		tokens:      tokens,
		vibes:       vibeStore,
		tagEnricher: enricher,
	}, nil
}

// SyncVibe fetches the user's top artists from Spotify, enriches them with
// tags, computes weighted vibe scores, and persists the result.
func (s *VibeService) SyncVibe(ctx context.Context, userID string) (*VibeSyncResult, error) {
	accessToken, err := s.ensureValidToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("authenticating with spotify: %w", err)
	}

	mediumResp, err := s.spotify.FetchTopArtists(ctx, accessToken, "medium_term", 50)
	if err != nil {
		return nil, fmt.Errorf("fetching medium-term top artists: %w", err)
	}

	shortResp, err := s.spotify.FetchTopArtists(ctx, accessToken, "short_term", 50)
	if err != nil {
		return nil, fmt.Errorf("fetching short-term top artists: %w", err)
	}

	// Build weighted artists and collect unique names.
	var weighted []vibes.WeightedArtist
	seen := make(map[string]bool)

	for i, a := range mediumResp.Items {
		rankWeight := float32(1.0) - float32(i)*0.02
		if rankWeight < 0 {
			rankWeight = 0
		}
		weighted = append(weighted, vibes.WeightedArtist{
			Name: a.Name, Weight: rankWeight * 1.0, // medium_term multiplier
		})
		seen[strings.ToLower(a.Name)] = true
	}
	for i, a := range shortResp.Items {
		rankWeight := float32(1.0) - float32(i)*0.02
		if rankWeight < 0 {
			rankWeight = 0
		}
		weighted = append(weighted, vibes.WeightedArtist{
			Name: a.Name, Weight: rankWeight * 0.5, // short_term multiplier
		})
		seen[strings.ToLower(a.Name)] = true
	}

	// Enrich artists with tags using a dedicated timeout.
	tagCtx, tagCancel := context.WithTimeout(context.WithoutCancel(ctx), configuration.VibeSyncTimeout)
	defer tagCancel()

	uniqueNames := make([]string, 0, len(seen))
	for name := range seen {
		uniqueNames = append(uniqueNames, name)
	}

	enrichResult := s.tagEnricher.Enrich(tagCtx, uniqueNames, configuration.LastFMRateLimit)
	weights := vibes.ComputeVibes(enrichResult.ArtistTags, weighted)

	// Use a fresh context for the DB write — the data is already in memory,
	// so we don't want the tag timeout to prevent saving results.
	dbCtx := context.WithoutCancel(ctx)
	if err := s.vibes.UpsertVibes(dbCtx, userID, weights); err != nil {
		return nil, fmt.Errorf("persisting vibe weights: %w", err)
	}

	observability.Logger(ctx).Info("vibe sync complete", "user", userID, "tags", len(weights))
	return &VibeSyncResult{VibeCount: len(weights)}, nil
}

// GetVibe returns the user's stored vibe weights.
func (s *VibeService) GetVibe(ctx context.Context, userID string) (map[string]float32, error) {
	return s.vibes.GetVibes(ctx, userID)
}

// ensureValidToken returns a valid access token, refreshing it if expired.
func (s *VibeService) ensureValidToken(ctx context.Context, userID string) (string, error) {
	tokens, err := s.tokens.GetTokens(ctx, userID)
	if err != nil {
		return "", err
	}

	if int64(tokens.TokenExpiry) <= time.Now().Unix()+configuration.TokenRefreshThreshold {
		refreshed, err := s.spotify.RefreshToken(ctx, tokens.RefreshToken)
		if err != nil {
			return "", err
		}

		newExpiry := int(time.Now().Unix()) + refreshed.ExpiresIn
		if err := s.tokens.UpdateTokens(ctx, userID, refreshed.AccessToken, refreshed.RefreshToken, newExpiry); err != nil {
			return "", err
		}

		return refreshed.AccessToken, nil
	}

	return tokens.AccessToken, nil
}
