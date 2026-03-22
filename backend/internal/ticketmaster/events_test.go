package ticketmaster

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchEvents_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("venueId") != "v1" {
			t.Errorf("venueId = %q, want v1", r.URL.Query().Get("venueId"))
		}
		if r.URL.Query().Get("classificationName") != "music" {
			t.Errorf("classificationName = %q, want music", r.URL.Query().Get("classificationName"))
		}
		if r.URL.Query().Get("sort") != "date,asc" {
			t.Errorf("sort = %q, want date,asc", r.URL.Query().Get("sort"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eventSearchResponse{
			Embedded: struct {
				Events []Event `json:"events"`
			}{
				Events: []Event{
					{
						ID:   "e1",
						Name: "Big Thief at Bowery Ballroom",
						Dates: EventDates{
							Start:  EventStart{DateTime: "2026-04-15T00:00:00Z", LocalDate: "2026-04-14"},
							Status: EventStatus{Code: "onsale"},
						},
						PriceRanges: []PriceRange{{Min: 35, Max: 55, Currency: "USD"}},
						Classifications: []Classification{
							{
								Segment:  NamedEntity{Name: "Music"},
								Genre:    NamedEntity{Name: "Rock"},
								SubGenre: NamedEntity{Name: "Alternative Rock"},
							},
						},
						Embedded: EventEmbedded{
							Venues: []Venue{{ID: "v1", Name: "Bowery Ballroom"}},
							Attractions: []Attraction{
								{ID: "a1", Name: "Big Thief", Classifications: []Classification{
									{Segment: NamedEntity{Name: "Music"}, Genre: NamedEntity{Name: "Rock"}},
								}},
							},
						},
					},
				},
			},
			Page: Page{Size: 200, TotalElements: 1, TotalPages: 1, Number: 0},
		})
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	events, err := c.SearchEvents(context.Background(), EventSearchOptions{VenueID: "v1"})
	if err != nil {
		t.Fatalf("SearchEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Name != "Big Thief at Bowery Ballroom" {
		t.Errorf("event name = %q, want Big Thief at Bowery Ballroom", events[0].Name)
	}
	if len(events[0].Embedded.Attractions) != 1 {
		t.Fatalf("expected 1 attraction, got %d", len(events[0].Embedded.Attractions))
	}
	if events[0].Embedded.Attractions[0].Name != "Big Thief" {
		t.Errorf("attraction = %q, want Big Thief", events[0].Embedded.Attractions[0].Name)
	}
	if events[0].Classifications[0].Genre.Name != "Rock" {
		t.Errorf("genre = %q, want Rock", events[0].Classifications[0].Genre.Name)
	}
}

func TestSearchEvents_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	_, err := c.SearchEvents(context.Background(), EventSearchOptions{VenueID: "v1"})
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestSearchEvents_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	_, err := c.SearchEvents(context.Background(), EventSearchOptions{VenueID: "v1"})
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Error("500 should not be ErrRateLimited")
	}
}

func TestSearchEvents_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(eventSearchResponse{
			Page: Page{Size: 200, TotalElements: 0, TotalPages: 0, Number: 0},
		})
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	events, err := c.SearchEvents(context.Background(), EventSearchOptions{VenueID: "v1"})
	if err != nil {
		t.Fatalf("SearchEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}
