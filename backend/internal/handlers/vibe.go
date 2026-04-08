package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/service"
)

// VibeManager provides vibe sync and read operations.
type VibeManager interface {
	SyncVibe(ctx context.Context, userID string) (*service.VibeSyncResult, error)
	GetVibe(ctx context.Context, userID string) (map[string]float32, error)
}

// VibeHandler handles HTTP vibe endpoints.
type VibeHandler struct {
	vibes VibeManager
}

func NewVibeHandler(vibes VibeManager) (*VibeHandler, error) {
	if vibes == nil {
		return nil, errors.New("vibe: nil vibe service")
	}
	return &VibeHandler{vibes: vibes}, nil
}

// SyncVibe triggers a vibe profile sync for the authenticated user.
func (h *VibeHandler) SyncVibe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	result, err := h.vibes.SyncVibe(r.Context(), claims.SpotifyID)
	if err != nil {
		httpError(w, r, http.StatusBadGateway, "failed to sync vibes",
			"vibe sync failed", "user", claims.SpotifyID, "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":     true,
		"vibe_count": result.VibeCount,
	})
}

// GetVibe returns the authenticated user's vibe weights.
func (h *VibeHandler) GetVibe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vibes, err := h.vibes.GetVibe(r.Context(), claims.SpotifyID)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to read vibe weights", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"vibes":      vibes,
		"vibe_count": len(vibes),
	})
}
