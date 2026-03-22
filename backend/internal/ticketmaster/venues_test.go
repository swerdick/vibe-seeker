package ticketmaster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchVenues_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("geoPoint") != "40.7128,-74.0060" {
			t.Errorf("geoPoint = %q, want 40.7128,-74.0060", r.URL.Query().Get("geoPoint"))
		}
		if r.URL.Query().Get("apikey") != "test-key" {
			t.Errorf("apikey = %q, want test-key", r.URL.Query().Get("apikey"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(venueSearchResponse{
			Embedded: struct {
				Venues []Venue `json:"venues"`
			}{
				Venues: []Venue{
					{ID: "v1", Name: "Bowery Ballroom", Location: VenueLocation{Latitude: "40.7204", Longitude: "-73.9934"}},
					{ID: "v2", Name: "Brooklyn Steel", Location: VenueLocation{Latitude: "40.7112", Longitude: "-73.9505"}},
				},
			},
			Page: Page{Size: 200, TotalElements: 2, TotalPages: 1, Number: 0},
		})
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	venues, err := c.SearchVenues(context.Background(), VenueSearchOptions{GeoPoint: "40.7128,-74.0060", Radius: "15"})
	if err != nil {
		t.Fatalf("SearchVenues failed: %v", err)
	}

	if len(venues) != 2 {
		t.Fatalf("expected 2 venues, got %d", len(venues))
	}
	if venues[0].Name != "Bowery Ballroom" {
		t.Errorf("first venue = %q, want Bowery Ballroom", venues[0].Name)
	}
}

func TestSearchVenues_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")

		var venues []Venue
		totalPages := 2
		if page == "0" {
			venues = []Venue{{ID: "v1", Name: "Venue 1"}}
		} else {
			venues = []Venue{{ID: "v2", Name: "Venue 2"}}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(venueSearchResponse{
			Embedded: struct {
				Venues []Venue `json:"venues"`
			}{Venues: venues},
			Page: Page{Size: 200, TotalElements: 2, TotalPages: totalPages, Number: callCount - 1},
		})
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	venues, err := c.SearchVenues(context.Background(), VenueSearchOptions{GeoPoint: "40.7128,-74.0060", Radius: "15"})
	if err != nil {
		t.Fatalf("SearchVenues failed: %v", err)
	}

	if len(venues) != 2 {
		t.Fatalf("expected 2 venues from 2 pages, got %d", len(venues))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestSearchVenues_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	_, err := c.SearchVenues(context.Background(), VenueSearchOptions{GeoPoint: "40.7128,-74.0060", Radius: "15"})
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}

func TestVenue_LatLng(t *testing.T) {
	v := Venue{Location: VenueLocation{Latitude: "40.7204", Longitude: "-73.9934"}}
	if v.Lat() != 40.7204 {
		t.Errorf("Lat() = %f, want 40.7204", v.Lat())
	}
	if v.Lng() != -73.9934 {
		t.Errorf("Lng() = %f, want -73.9934", v.Lng())
	}
}
