package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/service"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
)

type mockVenueService struct {
	syncResult     *service.VenueSyncResult
	syncErr        error
	syncVibesCount int
	syncVibesErr   error
	venues         []service.VenueWithDetails
	venuesErr      error
}

func (m *mockVenueService) SyncVenues(_ context.Context) (*service.VenueSyncResult, error) {
	return m.syncResult, m.syncErr
}

func (m *mockVenueService) SyncVenueVibes(_ context.Context) (int, error) {
	return m.syncVibesCount, m.syncVibesErr
}

func (m *mockVenueService) GetVenues(_ context.Context) ([]service.VenueWithDetails, error) {
	return m.venues, m.venuesErr
}

func TestSyncVenues_Success(t *testing.T) {
	h, err := NewVenueHandler(&mockVenueService{
		syncResult: &service.VenueSyncResult{
			Synced:        true,
			VenueCount:    1,
			ShowCount:     1,
			VibesComputed: 1,
		},
	})
	if err != nil {
		t.Fatalf("NewVenueHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/venues/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVenues(rec, req)

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
}

func TestSyncVenues_TTLSkip(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour)
	h, _ := NewVenueHandler(&mockVenueService{
		syncResult: &service.VenueSyncResult{
			Skipped:     true,
			LastFetched: &recent,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/venues/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVenues(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["synced"] != false {
		t.Error("expected synced=false when data is fresh")
	}
}

func TestSyncVenues_Unauthorized(t *testing.T) {
	h, _ := NewVenueHandler(&mockVenueService{})

	req := httptest.NewRequest(http.MethodPost, "/api/venues/sync", nil)
	rec := httptest.NewRecorder()

	h.SyncVenues(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSyncVenues_ServiceError(t *testing.T) {
	h, _ := NewVenueHandler(&mockVenueService{
		syncErr: errors.New("ticketmaster down"),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/venues/sync", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVenues(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetVenues_Success(t *testing.T) {
	h, _ := NewVenueHandler(&mockVenueService{
		venues: []service.VenueWithDetails{
			{
				Venue: store.Venue{ID: "tm_v1", Name: "Bowery Ballroom", Latitude: 40.7204, Longitude: -73.9934, ShowsTracked: 5},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/venues", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.GetVenues(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["count"] != float64(1) {
		t.Errorf("expected count=1, got %v", body["count"])
	}
}

func TestGetVenues_Unauthorized(t *testing.T) {
	h, _ := NewVenueHandler(&mockVenueService{})

	req := httptest.NewRequest(http.MethodGet, "/api/venues", nil)
	rec := httptest.NewRecorder()

	h.GetVenues(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSyncVenueVibes_Success(t *testing.T) {
	h, err := NewVenueHandler(&mockVenueService{
		syncVibesCount: 3,
	})
	if err != nil {
		t.Fatalf("NewVenueHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/venues/vibes", nil)
	req = addTestClaims(t, req)
	rec := httptest.NewRecorder()

	h.SyncVenueVibes(rec, req)

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
}

func TestSyncVenueVibes_Unauthorized(t *testing.T) {
	h, _ := NewVenueHandler(&mockVenueService{})

	req := httptest.NewRequest(http.MethodPost, "/api/venues/vibes", nil)
	rec := httptest.NewRecorder()

	h.SyncVenueVibes(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMapVenues_FiltersNonUS(t *testing.T) {
	tmVenues := []ticketmaster.Venue{
		{
			ID:       "v1",
			Name:     "Bowery Ballroom",
			Location: ticketmaster.VenueLocation{Latitude: "40.7204", Longitude: "-73.9934"},
			City:     ticketmaster.VenueCity{Name: "New York"},
			State:    ticketmaster.VenueState{StateCode: "NY"},
			Country:  ticketmaster.VenueCountry{CountryCode: "US"},
		},
		{
			ID:       "v2",
			Name:     "Spanish Rec Center",
			Location: ticketmaster.VenueLocation{Latitude: "40.7128", Longitude: "-74.0060"},
			City:     ticketmaster.VenueCity{Name: "Donostia"},
			State:    ticketmaster.VenueState{StateCode: ""},
			Country:  ticketmaster.VenueCountry{CountryCode: "ES"},
		},
		{
			ID:       "v3",
			Name:     "No Coordinates",
			Location: ticketmaster.VenueLocation{Latitude: "0", Longitude: "0"},
			Country:  ticketmaster.VenueCountry{CountryCode: "US"},
		},
	}

	result := service.MapVenues(tmVenues)
	if len(result) != 1 {
		t.Fatalf("expected 1 venue after filtering, got %d", len(result))
	}
	if result[0].Name != "Bowery Ballroom" {
		t.Errorf("expected Bowery Ballroom, got %q", result[0].Name)
	}
}

func TestMapVenues_EmptyCountryAllowed(t *testing.T) {
	tmVenues := []ticketmaster.Venue{
		{
			ID:       "v1",
			Name:     "Unknown Country Venue",
			Location: ticketmaster.VenueLocation{Latitude: "40.7", Longitude: "-74.0"},
			Country:  ticketmaster.VenueCountry{CountryCode: ""},
		},
	}

	result := service.MapVenues(tmVenues)
	if len(result) != 1 {
		t.Fatalf("expected 1 venue (empty country allowed), got %d", len(result))
	}
}
