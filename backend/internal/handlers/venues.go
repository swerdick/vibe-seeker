package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/service"
)

// VenueManager provides venue sync, vibe computation, and listing operations.
type VenueManager interface {
	SyncVenues(ctx context.Context) (*service.VenueSyncResult, error)
	SyncVenueVibes(ctx context.Context) (int, error)
	GetVenues(ctx context.Context) ([]service.VenueWithDetails, error)
}

// VenueHandler handles HTTP venue endpoints.
type VenueHandler struct {
	venues VenueManager
}

func NewVenueHandler(venues VenueManager) (*VenueHandler, error) {
	if venues == nil {
		return nil, errors.New("venues: nil venue service")
	}
	return &VenueHandler{venues: venues}, nil
}

// SyncVenues triggers a venue and event sync from Ticketmaster.
func (h *VenueHandler) SyncVenues(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.venues.SyncVenues(r.Context())
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"venue sync failed", "error", err)
		return
	}

	if result.Skipped {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"synced":       false,
			"reason":       "data is fresh",
			"last_fetched": result.LastFetched.Format(time.RFC3339),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":         true,
		"venues_count":   result.VenueCount,
		"shows_count":    result.ShowCount,
		"vibes_computed": result.VibesComputed,
	})
}

// SyncVenueVibes recomputes vibe profiles for all venues without
// re-fetching from Ticketmaster.
func (h *VenueHandler) SyncVenueVibes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	computed, err := h.venues.SyncVenueVibes(r.Context())
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"venue vibe sync failed", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":         true,
		"vibes_computed": computed,
	})
}

// GetVenues returns all cached venues with their upcoming shows and vibes.
func (h *VenueHandler) GetVenues(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	venues, err := h.venues.GetVenues(r.Context())
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to read venues", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"venues": venues,
		"count":  len(venues),
	})
}
