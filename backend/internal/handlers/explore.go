package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

// VibeExplorer provides tag exploration operations.
type VibeExplorer interface {
	GetTopVibes(ctx context.Context, limit int) ([]store.TagPrevalence, error)
	GetRelatedVibes(ctx context.Context, tag string, limit int) ([]store.TagRelation, error)
}

type ExploreHandler struct {
	explore VibeExplorer
}

func NewExploreHandler(explore VibeExplorer) (*ExploreHandler, error) {
	if explore == nil {
		return nil, errors.New("explore: nil explore service")
	}
	return &ExploreHandler{explore: explore}, nil
}

// GetTopVibes returns the most prevalent vibes across all cached artist data.
// Query params: limit (default 10, max 500).
func (h *ExploreHandler) GetTopVibes(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	tags, err := h.explore.GetTopVibes(r.Context(), limit)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "failed to fetch vibes",
			"failed to query top tags", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"vibes": tags,
	})
}

// GetRelatedVibes returns vibes related to a given tag by artist co-occurrence.
// Query params: tag (required), limit (default 8, max 20).
func (h *ExploreHandler) GetRelatedVibes(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag parameter", http.StatusBadRequest)
		return
	}

	limit := 8
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}

	related, err := h.explore.GetRelatedVibes(r.Context(), tag, limit)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "failed to fetch related vibes",
			"failed to query related tags", "error", err, "tag", tag)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tag":     tag,
		"related": related,
	})
}
