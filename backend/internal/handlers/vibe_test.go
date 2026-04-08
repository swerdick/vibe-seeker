package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/service"
)

type mockVibeService struct {
	syncResult *service.VibeSyncResult
	syncErr    error
	getVibes   map[string]float32
	getErr     error
}

func (m *mockVibeService) SyncVibe(_ context.Context, _ string) (*service.VibeSyncResult, error) {
	return m.syncResult, m.syncErr
}

func (m *mockVibeService) GetVibe(_ context.Context, _ string) (map[string]float32, error) {
	return m.getVibes, m.getErr
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

func TestSyncVibe_Success(t *testing.T) {
	h, err := NewVibeHandler(&mockVibeService{
		syncResult: &service.VibeSyncResult{VibeCount: 5},
	})
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

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["synced"] != true {
		t.Error("expected synced=true")
	}
	if body["vibe_count"] != float64(5) {
		t.Errorf("expected vibe_count=5, got %v", body["vibe_count"])
	}
}

func TestSyncVibe_Unauthorized(t *testing.T) {
	h, _ := NewVibeHandler(&mockVibeService{})

	req := httptest.NewRequest(http.MethodPost, "/api/vibe/sync", nil)
	rec := httptest.NewRecorder()

	h.SyncVibe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSyncVibe_ServiceError(t *testing.T) {
	h, _ := NewVibeHandler(&mockVibeService{
		syncErr: errors.New("spotify down"),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/vibe/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVibe(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
}

func TestGetVibe_Success(t *testing.T) {
	h, _ := NewVibeHandler(&mockVibeService{
		getVibes: map[string]float32{"rock": 1.0, "indie": 0.7},
	})

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

	vibes, ok := body["vibes"].(map[string]interface{})
	if !ok {
		t.Fatal("expected genres map in response")
	}
	if vibes["rock"] == nil {
		t.Error("expected 'rock' in vibes")
	}
}

func TestGetVibe_Unauthorized(t *testing.T) {
	h, _ := NewVibeHandler(&mockVibeService{})

	req := httptest.NewRequest(http.MethodGet, "/api/vibe", nil)
	rec := httptest.NewRecorder()

	h.GetVibe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
