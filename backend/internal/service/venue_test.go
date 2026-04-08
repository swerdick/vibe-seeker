package service

import (
	"context"
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
)

type mockTicketmaster struct {
	venues []ticketmaster.Venue
	events []ticketmaster.Event
	err    error
}

func (m *mockTicketmaster) SearchVenues(_ context.Context, _ ticketmaster.VenueSearchOptions) ([]ticketmaster.Venue, error) {
	return m.venues, m.err
}

func (m *mockTicketmaster) SearchEvents(_ context.Context, _ ticketmaster.EventSearchOptions) ([]ticketmaster.Event, error) {
	return m.events, m.err
}

type mockVenueStore2 struct {
	upsertVenuesCalled          bool
	upsertShowsCalled           bool
	upsertArtistsCalled         bool
	upsertShowArtistsCalled     bool
	upsertClassificationsCalled bool
	upsertVibesCalled           bool
	venues                      []store.Venue
	fetchedAt                   *time.Time
	showsByVenue                map[string][]store.ShowSummary
	artistsByVenue              map[string][]store.VenueArtist
	vibesByVenue                map[string]map[string]float32
	err                         error
}

func (m *mockVenueStore2) UpsertVenues(_ context.Context, _ []store.Venue) error {
	m.upsertVenuesCalled = true
	return m.err
}
func (m *mockVenueStore2) UpsertShows(_ context.Context, _ []store.Show) error {
	m.upsertShowsCalled = true
	return m.err
}
func (m *mockVenueStore2) UpsertArtists(_ context.Context, _ []store.Artist) error {
	m.upsertArtistsCalled = true
	return m.err
}
func (m *mockVenueStore2) UpsertShowArtists(_ context.Context, _ []store.ShowArtist) error {
	m.upsertShowArtistsCalled = true
	return m.err
}
func (m *mockVenueStore2) UpsertShowClassifications(_ context.Context, _ []store.ShowClassification) error {
	m.upsertClassificationsCalled = true
	return m.err
}
func (m *mockVenueStore2) GetVenueFetchedAt(_ context.Context, _ string) (*time.Time, error) {
	return m.fetchedAt, nil
}
func (m *mockVenueStore2) GetVenues(_ context.Context) ([]store.Venue, error) {
	return m.venues, m.err
}
func (m *mockVenueStore2) GetShowsForVenues(_ context.Context, _ []string) (map[string][]store.ShowSummary, error) {
	return m.showsByVenue, nil
}
func (m *mockVenueStore2) GetAllVenueArtists(_ context.Context, _ []string) (map[string][]store.VenueArtist, error) {
	return m.artistsByVenue, nil
}
func (m *mockVenueStore2) GetAllVenueVibes(_ context.Context, _ []string) (map[string]map[string]float32, error) {
	return m.vibesByVenue, nil
}
func (m *mockVenueStore2) UpsertVenueVibes(_ context.Context, _ string, _ map[string]float32) error {
	m.upsertVibesCalled = true
	return nil
}

func TestSyncVenues_TTLSkip(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour)
	vs := &mockVenueStore2{fetchedAt: &recent}

	cache := newMockTagCache()
	enricher, _ := NewTagEnricher(&mockLastFM{}, cache)

	svc, err := NewVenueService(&mockTicketmaster{}, vs, enricher)
	if err != nil {
		t.Fatalf("NewVenueService: %v", err)
	}

	result, err := svc.SyncVenues(context.Background())
	if err != nil {
		t.Fatalf("SyncVenues: %v", err)
	}

	if !result.Skipped {
		t.Error("expected sync to be skipped when data is fresh")
	}
	if vs.upsertVenuesCalled {
		t.Error("expected UpsertVenues NOT to be called")
	}
}

func TestSyncVenueVibes_Success(t *testing.T) {
	vs := &mockVenueStore2{
		venues: []store.Venue{
			{ID: "v1", Name: "Test Venue", ShowsTracked: 2},
		},
		artistsByVenue: map[string][]store.VenueArtist{
			"v1": {
				{ArtistName: "artist one", ShowDate: time.Now()},
			},
		},
	}

	cache := newMockTagCache()
	cache.cached["artist one"] = []lastfm.Tag{{Name: "rock", Count: 100}}
	enricher, _ := NewTagEnricher(&mockLastFM{}, cache)

	svc, err := NewVenueService(&mockTicketmaster{}, vs, enricher)
	if err != nil {
		t.Fatalf("NewVenueService: %v", err)
	}

	computed, err := svc.SyncVenueVibes(context.Background())
	if err != nil {
		t.Fatalf("SyncVenueVibes: %v", err)
	}

	if computed != 1 {
		t.Errorf("expected 1 venue vibe computed, got %d", computed)
	}
	if !vs.upsertVibesCalled {
		t.Error("expected UpsertVenueVibes to be called")
	}
}

func TestGetVenues_Success(t *testing.T) {
	vs := &mockVenueStore2{
		venues: []store.Venue{
			{ID: "v1", Name: "Venue A", ShowsTracked: 3},
			{ID: "v2", Name: "Venue B", ShowsTracked: 0}, // no shows, filtered
		},
		showsByVenue: map[string][]store.ShowSummary{
			"v1": {{Name: "Show 1", Date: time.Now()}},
		},
		vibesByVenue: map[string]map[string]float32{
			"v1": {"rock": 0.9},
		},
	}

	cache := newMockTagCache()
	enricher, _ := NewTagEnricher(&mockLastFM{}, cache)

	svc, _ := NewVenueService(&mockTicketmaster{}, vs, enricher)

	venues, err := svc.GetVenues(context.Background())
	if err != nil {
		t.Fatalf("GetVenues: %v", err)
	}

	if len(venues) != 1 {
		t.Fatalf("expected 1 venue (with shows), got %d", len(venues))
	}
	if venues[0].Name != "Venue A" {
		t.Errorf("expected 'Venue A', got %q", venues[0].Name)
	}
	if len(venues[0].Shows) != 1 {
		t.Errorf("expected 1 show, got %d", len(venues[0].Shows))
	}
	if venues[0].Vibes["rock"] != 0.9 {
		t.Errorf("expected rock=0.9, got %f", venues[0].Vibes["rock"])
	}
}

func TestGetVenues_Empty(t *testing.T) {
	vs := &mockVenueStore2{venues: []store.Venue{}}
	cache := newMockTagCache()
	enricher, _ := NewTagEnricher(&mockLastFM{}, cache)

	svc, _ := NewVenueService(&mockTicketmaster{}, vs, enricher)

	venues, err := svc.GetVenues(context.Background())
	if err != nil {
		t.Fatalf("GetVenues: %v", err)
	}

	if len(venues) != 0 {
		t.Errorf("expected 0 venues, got %d", len(venues))
	}
}

func TestNewVenueService_NilDeps(t *testing.T) {
	cache := newMockTagCache()
	enricher, _ := NewTagEnricher(&mockLastFM{}, cache)
	tm := &mockTicketmaster{}
	vs := &mockVenueStore2{}

	if _, err := NewVenueService(nil, vs, enricher); err == nil {
		t.Error("expected error for nil ticketmaster")
	}
	if _, err := NewVenueService(tm, nil, enricher); err == nil {
		t.Error("expected error for nil venue store")
	}
	if _, err := NewVenueService(tm, vs, nil); err == nil {
		t.Error("expected error for nil tag enricher")
	}
}

func TestMapVenues_FiltersNonUS(t *testing.T) {
	tmVenues := []ticketmaster.Venue{
		{
			ID:       "v1",
			Name:     "Bowery Ballroom",
			Location: ticketmaster.VenueLocation{Latitude: "40.7204", Longitude: "-73.9934"},
			Country:  ticketmaster.VenueCountry{CountryCode: "US"},
		},
		{
			ID:       "v2",
			Name:     "Spanish Venue",
			Location: ticketmaster.VenueLocation{Latitude: "40.7", Longitude: "-74.0"},
			Country:  ticketmaster.VenueCountry{CountryCode: "ES"},
		},
	}

	result := MapVenues(tmVenues)
	if len(result) != 1 {
		t.Fatalf("expected 1 venue, got %d", len(result))
	}
	if result[0].Name != "Bowery Ballroom" {
		t.Errorf("expected Bowery Ballroom, got %q", result[0].Name)
	}
}

func TestMapVenues_FiltersNoCoordinates(t *testing.T) {
	tmVenues := []ticketmaster.Venue{
		{
			ID:       "v1",
			Name:     "No Coords",
			Location: ticketmaster.VenueLocation{Latitude: "0", Longitude: "0"},
			Country:  ticketmaster.VenueCountry{CountryCode: "US"},
		},
	}

	result := MapVenues(tmVenues)
	if len(result) != 0 {
		t.Fatalf("expected 0 venues (no coords), got %d", len(result))
	}
}
