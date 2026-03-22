package lastfm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchArtistTags_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("method") != "artist.gettoptags" {
			t.Errorf("method = %q, want artist.gettoptags", r.URL.Query().Get("method"))
		}
		if r.URL.Query().Get("artist") != "Big Thief" {
			t.Errorf("artist = %q, want Big Thief", r.URL.Query().Get("artist"))
		}
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("format = %q, want json", r.URL.Query().Get("format"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(topTagsResponse{
			TopTags: struct {
				Tag []Tag `json:"tag"`
			}{
				Tag: []Tag{
					{Name: "indie", Count: 100},
					{Name: "folk", Count: 75},
					{Name: "seen live", Count: 60},
					{Name: "noise", Count: 10},
				},
			},
		})
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL + "/"

	tags, err := c.FetchArtistTags(context.Background(), "Big Thief")
	if err != nil {
		t.Fatalf("FetchArtistTags failed: %v", err)
	}

	// "seen live" should be blocklisted, "noise" (count=10) should be filtered by threshold.
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags after filtering, got %d: %+v", len(tags), tags)
	}
	if tags[0].Name != "indie" {
		t.Errorf("first tag = %q, want indie", tags[0].Name)
	}
	if tags[1].Name != "folk" {
		t.Errorf("second tag = %q, want folk", tags[1].Name)
	}
}

func TestFetchArtistTags_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL + "/"

	_, err := c.FetchArtistTags(context.Background(), "Unknown")
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}

func TestFetchArtistTags_EmptyTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(topTagsResponse{})
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL + "/"

	tags, err := c.FetchArtistTags(context.Background(), "Nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}
}

func TestFilterTags_BlocklistCaseInsensitive(t *testing.T) {
	tags := []Tag{
		{Name: "Seen Live", Count: 90},
		{Name: "rock", Count: 80},
	}

	filtered := filterTags(tags)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(filtered))
	}
	if filtered[0].Name != "rock" {
		t.Errorf("expected rock, got %q", filtered[0].Name)
	}
}
