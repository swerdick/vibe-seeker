package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTopArtists_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer mock-token" {
			t.Errorf("Authorization = %q, want Bearer mock-token", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("time_range") != "medium_term" {
			t.Errorf("time_range = %q, want medium_term", r.URL.Query().Get("time_range"))
		}
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("limit = %q, want 50", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TopArtistsResponse{
			Items: []Artist{
				{ID: "a1", Name: "Artist One"},
				{ID: "a2", Name: "Artist Two"},
			},
		})
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TopArtistsURL = server.URL

	resp, err := c.FetchTopArtists(context.Background(),"mock-token", "medium_term", 50)
	if err != nil {
		t.Fatalf("FetchTopArtists failed: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 artists, got %d", len(resp.Items))
	}
	if resp.Items[0].Name != "Artist One" {
		t.Errorf("first artist = %q, want Artist One", resp.Items[0].Name)
	}
}

func TestFetchTopArtists_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TopArtistsURL = server.URL

	_, err := c.FetchTopArtists(context.Background(),"bad-token", "medium_term", 50)
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}
