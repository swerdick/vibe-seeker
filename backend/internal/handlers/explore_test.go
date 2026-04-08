package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

type mockExploreService struct {
	topVibes    []store.TagPrevalence
	topErr      error
	relatedTags []store.TagRelation
	relatedErr  error
	lastTag     string
	lastLimit   int
}

func (m *mockExploreService) GetTopVibes(_ context.Context, limit int) ([]store.TagPrevalence, error) {
	m.lastLimit = limit
	return m.topVibes, m.topErr
}

func (m *mockExploreService) GetRelatedVibes(_ context.Context, tag string, limit int) ([]store.TagRelation, error) {
	m.lastTag = tag
	m.lastLimit = limit
	return m.relatedTags, m.relatedErr
}

func TestGetTopVibes_DefaultLimit(t *testing.T) {
	mock := &mockExploreService{
		topVibes: []store.TagPrevalence{
			{Tag: "rock", Prevalence: 1.0},
			{Tag: "indie", Prevalence: 0.85},
		},
	}

	h, err := NewExploreHandler(mock)
	if err != nil {
		t.Fatalf("NewExploreHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/vibes/top", nil)
	rec := httptest.NewRecorder()

	h.GetTopVibes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mock.lastLimit != 10 {
		t.Errorf("expected default limit 10, got %d", mock.lastLimit)
	}

	var body struct {
		Vibes []store.TagPrevalence `json:"vibes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Vibes) != 2 {
		t.Errorf("expected 2 vibes, got %d", len(body.Vibes))
	}
	if body.Vibes[0].Tag != "rock" {
		t.Errorf("first vibe tag = %q, want %q", body.Vibes[0].Tag, "rock")
	}
}

func TestGetTopVibes_CustomLimit(t *testing.T) {
	mock := &mockExploreService{topVibes: []store.TagPrevalence{}}
	h, _ := NewExploreHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/vibes/top?limit=25", nil)
	rec := httptest.NewRecorder()

	h.GetTopVibes(rec, req)

	if mock.lastLimit != 25 {
		t.Errorf("expected limit 25, got %d", mock.lastLimit)
	}
}

func TestGetTopVibes_LimitCapped(t *testing.T) {
	mock := &mockExploreService{topVibes: []store.TagPrevalence{}}
	h, _ := NewExploreHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/vibes/top?limit=999", nil)
	rec := httptest.NewRecorder()

	h.GetTopVibes(rec, req)

	// Invalid limit (>500) should fall back to default.
	if mock.lastLimit != 10 {
		t.Errorf("expected default limit 10 for out-of-range, got %d", mock.lastLimit)
	}
}

func TestGetRelatedVibes_Success(t *testing.T) {
	mock := &mockExploreService{
		relatedTags: []store.TagRelation{
			{Tag: "indie", Strength: 1.0},
			{Tag: "alternative", Strength: 0.75},
		},
	}
	h, _ := NewExploreHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/vibes/related?tag=rock&limit=5", nil)
	rec := httptest.NewRecorder()

	h.GetRelatedVibes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mock.lastTag != "rock" {
		t.Errorf("expected tag %q, got %q", "rock", mock.lastTag)
	}
	if mock.lastLimit != 5 {
		t.Errorf("expected limit 5, got %d", mock.lastLimit)
	}

	var body struct {
		Tag     string              `json:"tag"`
		Related []store.TagRelation `json:"related"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Tag != "rock" {
		t.Errorf("response tag = %q, want %q", body.Tag, "rock")
	}
	if len(body.Related) != 2 {
		t.Errorf("expected 2 related vibes, got %d", len(body.Related))
	}
}

func TestGetRelatedVibes_MissingTag(t *testing.T) {
	h, _ := NewExploreHandler(&mockExploreService{})

	req := httptest.NewRequest(http.MethodGet, "/api/vibes/related", nil)
	rec := httptest.NewRecorder()

	h.GetRelatedVibes(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestNewExploreHandler_NilService(t *testing.T) {
	_, err := NewExploreHandler(nil)
	if err == nil {
		t.Error("expected error for nil explore service")
	}
}
