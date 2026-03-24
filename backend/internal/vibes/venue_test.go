package vibes

import (
	"testing"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

func TestComputeVenueVibe_Basic(t *testing.T) {
	now := time.Now()
	artists := []VenueArtist{
		{ArtistName: "Artist One", ShowDate: now.Add(-24 * time.Hour)},       // recent
		{ArtistName: "Artist Two", ShowDate: now.Add(-200 * 24 * time.Hour)}, // old
	}

	artistTags := map[string][]lastfm.Tag{
		"artist one": {
			{Name: "rock", Count: 100},
			{Name: "indie", Count: 80},
		},
		"artist two": {
			{Name: "jazz", Count: 100},
		},
	}

	vibes := ComputeVenueVibe(artists, artistTags)

	// "rock" from a recent artist should dominate.
	if vibes["rock"] != 1.0 {
		t.Errorf("rock = %f, want 1.0 (recent artist)", vibes["rock"])
	}

	// "jazz" from an old artist should be present but lower.
	if vibes["jazz"] >= vibes["rock"] {
		t.Errorf("jazz (%f) should be less than rock (%f)", vibes["jazz"], vibes["rock"])
	}
}

func TestComputeVenueVibe_DuplicateArtist(t *testing.T) {
	now := time.Now()
	// Same artist played twice — should use the most recent show's recency weight.
	artists := []VenueArtist{
		{ArtistName: "Repeat Act", ShowDate: now.Add(-24 * time.Hour)},       // recent
		{ArtistName: "Repeat Act", ShowDate: now.Add(-300 * 24 * time.Hour)}, // old
	}

	artistTags := map[string][]lastfm.Tag{
		"repeat act": {{Name: "punk", Count: 100}},
	}

	vibes := ComputeVenueVibe(artists, artistTags)

	// Should use the recent show's weight (1.0), not the old one (0.4).
	if vibes["punk"] != 1.0 {
		t.Errorf("punk = %f, want 1.0 (should use most recent show)", vibes["punk"])
	}
}

func TestComputeVenueVibe_Empty(t *testing.T) {
	vibes := ComputeVenueVibe(nil, nil)
	if len(vibes) != 0 {
		t.Errorf("expected empty vibes, got %d entries", len(vibes))
	}
}

func TestComputeVenueVibe_NoTags(t *testing.T) {
	artists := []VenueArtist{
		{ArtistName: "Unknown", ShowDate: time.Now()},
	}

	vibes := ComputeVenueVibe(artists, map[string][]lastfm.Tag{})
	if len(vibes) != 0 {
		t.Errorf("expected empty vibes for untagged artist, got %d entries", len(vibes))
	}
}
