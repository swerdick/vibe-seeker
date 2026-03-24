package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
)

type mockVenueStore struct {
	upsertVenuesCalled          bool
	upsertShowsCalled           bool
	upsertArtistsCalled         bool
	upsertShowArtistsCalled     bool
	upsertClassificationsCalled bool
	venues                      []store.Venue
	fetchedAt                   *time.Time
	err                         error
}

func (m *mockVenueStore) UpsertVenues(_ context.Context, _ []store.Venue) error {
	m.upsertVenuesCalled = true
	return m.err
}
func (m *mockVenueStore) UpsertShows(_ context.Context, _ []store.Show) error {
	m.upsertShowsCalled = true
	return m.err
}
func (m *mockVenueStore) UpsertArtists(_ context.Context, _ []store.Artist) error {
	m.upsertArtistsCalled = true
	return m.err
}
func (m *mockVenueStore) UpsertShowArtists(_ context.Context, _ []store.ShowArtist) error {
	m.upsertShowArtistsCalled = true
	return m.err
}
func (m *mockVenueStore) UpsertShowClassifications(_ context.Context, _ []store.ShowClassification) error {
	m.upsertClassificationsCalled = true
	return m.err
}
func (m *mockVenueStore) GetVenueFetchedAt(_ context.Context, _ string) (*time.Time, error) {
	return m.fetchedAt, nil
}
func (m *mockVenueStore) GetVenues(_ context.Context) ([]store.Venue, error) {
	return m.venues, m.err
}
func (m *mockVenueStore) GetShowsForVenues(_ context.Context, _ []string) (map[string][]store.ShowSummary, error) {
	return nil, nil
}
func (m *mockVenueStore) GetAllVenueArtists(_ context.Context, _ []string) (map[string][]store.VenueArtist, error) {
	return nil, nil
}
func (m *mockVenueStore) GetAllVenueVibes(_ context.Context, _ []string) (map[string]map[string]float32, error) {
	return nil, nil
}
func (m *mockVenueStore) UpsertVenueVibes(_ context.Context, _ string, _ map[string]float32) error {
	return nil
}

func newMockLastFMForVenues() (*lastfm.Client, *mockTagCache) {
	lfm := lastfm.NewClient("test-key")
	return lfm, &mockTagCache{}
}

func newMockTMServer(t *testing.T) (*ticketmaster.Client, *httptest.Server) {
	t.Helper()

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/venues.json":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"_embedded": map[string]interface{}{
					"venues": []map[string]interface{}{
						{
							"id": "v1", "name": "Bowery Ballroom",
							"location": map[string]string{"latitude": "40.7204", "longitude": "-73.9934"},
							"address":  map[string]string{"line1": "6 Delancey St"},
							"city":     map[string]string{"name": "New York"},
							"state":    map[string]string{"stateCode": "NY"},
						},
					},
				},
				"page": map[string]int{"size": 200, "totalElements": 1, "totalPages": 1, "number": 0},
			})
		case "/events.json":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"_embedded": map[string]interface{}{
					"events": []map[string]interface{}{
						{
							"id": "e1", "name": "Big Thief Live",
							"dates": map[string]interface{}{
								"start":  map[string]string{"dateTime": "2026-04-15T00:00:00Z"},
								"status": map[string]string{"code": "onsale"},
							},
							"priceRanges": []map[string]interface{}{
								{"min": 35.0, "max": 55.0, "currency": "USD"},
							},
							"classifications": []map[string]interface{}{
								{
									"segment":  map[string]string{"name": "Music"},
									"genre":    map[string]string{"name": "Rock"},
									"subGenre": map[string]string{"name": "Alternative Rock"},
								},
							},
							"_embedded": map[string]interface{}{
								"venues":      []map[string]interface{}{{"id": "v1", "name": "Bowery Ballroom"}},
								"attractions": []map[string]interface{}{{"id": "a1", "name": "Big Thief"}},
							},
						},
					},
				},
				"page": map[string]int{"size": 200, "totalElements": 1, "totalPages": 1, "number": 0},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	c := ticketmaster.NewClient("test-key")
	c.BaseURL = mock.URL
	c.HTTPClient = mock.Client()

	return c, mock
}

func TestSyncVenues_Success(t *testing.T) {
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	venueStore := &mockVenueStore{}
	h, err := NewVenueHandler(tm, lfm, venueStore, tc)
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

	if !venueStore.upsertVenuesCalled {
		t.Error("expected UpsertVenues to be called")
	}
	if !venueStore.upsertShowsCalled {
		t.Error("expected UpsertShows to be called")
	}
	if !venueStore.upsertArtistsCalled {
		t.Error("expected UpsertArtists to be called")
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
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	recent := time.Now().Add(-1 * time.Hour)
	venueStore := &mockVenueStore{fetchedAt: &recent}
	h, _ := NewVenueHandler(tm, lfm, venueStore, tc)

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
	if venueStore.upsertVenuesCalled {
		t.Error("expected UpsertVenues NOT to be called when data is fresh")
	}
}

func TestSyncVenues_Unauthorized(t *testing.T) {
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	h, _ := NewVenueHandler(tm, lfm, &mockVenueStore{}, tc)

	req := httptest.NewRequest(http.MethodPost, "/api/venues/sync", nil)
	rec := httptest.NewRecorder()

	h.SyncVenues(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetVenues_Success(t *testing.T) {
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	venueStore := &mockVenueStore{
		venues: []store.Venue{
			{ID: "tm_v1", Name: "Bowery Ballroom", Latitude: 40.7204, Longitude: -73.9934, ShowsTracked: 5},
		},
	}
	h, _ := NewVenueHandler(tm, lfm, venueStore, tc)

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
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	h, _ := NewVenueHandler(tm, lfm, &mockVenueStore{}, tc)

	req := httptest.NewRequest(http.MethodGet, "/api/venues", nil)
	rec := httptest.NewRecorder()

	h.GetVenues(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSyncVenueVibes_Success(t *testing.T) {
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	venueStore := &mockVenueStore{
		venues: []store.Venue{
			{ID: "tm_v1", Name: "Test Venue", ShowsTracked: 5},
		},
	}
	h, err := NewVenueHandler(tm, lfm, venueStore, tc)
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
	tm, mock := newMockTMServer(t)
	defer mock.Close()
	lfm, tc := newMockLastFMForVenues()

	h, _ := NewVenueHandler(tm, lfm, &mockVenueStore{}, tc)

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

	result := mapVenues(tmVenues)
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

	result := mapVenues(tmVenues)
	if len(result) != 1 {
		t.Fatalf("expected 1 venue (empty country allowed), got %d", len(result))
	}
}
