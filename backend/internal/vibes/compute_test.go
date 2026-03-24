package vibes

import (
	"math"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

func TestComputeVibes_Basic(t *testing.T) {
	artistTags := map[string][]lastfm.Tag{
		"artist one": {
			{Name: "rock", Count: 100},
			{Name: "indie", Count: 80},
		},
		"artist two": {
			{Name: "rock", Count: 90},
			{Name: "alternative", Count: 60},
		},
	}

	artists := []WeightedArtist{
		{Name: "Artist One", Weight: 1.0},
		{Name: "Artist Two", Weight: 0.8},
	}

	weights := ComputeVibes(artistTags, artists)

	if weights["rock"] != 1.0 {
		t.Errorf("rock weight = %f, want 1.0", weights["rock"])
	}

	for _, tag := range []string{"indie", "alternative"} {
		w, ok := weights[tag]
		if !ok {
			t.Errorf("expected tag %q in weights", tag)
			continue
		}
		if w <= 0 || w > 1.0 {
			t.Errorf("%s weight = %f, want in (0, 1]", tag, w)
		}
	}
}

func TestComputeVibes_Empty(t *testing.T) {
	weights := ComputeVibes(nil, nil)
	if len(weights) != 0 {
		t.Errorf("expected empty weights, got %d entries", len(weights))
	}
}

func TestComputeVibes_MissingArtist(t *testing.T) {
	weights := ComputeVibes(
		map[string][]lastfm.Tag{},
		[]WeightedArtist{{Name: "Unknown", Weight: 1.0}},
	)
	if len(weights) != 0 {
		t.Errorf("expected empty weights, got %d entries", len(weights))
	}
}

func TestComputeVibes_Normalization(t *testing.T) {
	artistTags := map[string][]lastfm.Tag{
		"a": {{Name: "x", Count: 100}, {Name: "y", Count: 50}},
		"b": {{Name: "x", Count: 80}},
	}
	artists := []WeightedArtist{
		{Name: "A", Weight: 1.0},
		{Name: "B", Weight: 0.5},
	}

	weights := ComputeVibes(artistTags, artists)

	var max float32
	for _, w := range weights {
		if w > max {
			max = w
		}
	}
	if math.Abs(float64(max)-1.0) > 0.001 {
		t.Errorf("max weight = %f, want 1.0", max)
	}
}

func TestComputeVibes_WeightInfluence(t *testing.T) {
	artistTags := map[string][]lastfm.Tag{
		"heavy":  {{Name: "heavy-tag", Count: 100}},
		"light": {{Name: "light-tag", Count: 100}},
	}
	artists := []WeightedArtist{
		{Name: "heavy", Weight: 1.0},
		{Name: "light", Weight: 0.1},
	}

	weights := ComputeVibes(artistTags, artists)
	if weights["heavy-tag"] <= weights["light-tag"] {
		t.Errorf("heavy-tag (%f) should have more weight than light-tag (%f)",
			weights["heavy-tag"], weights["light-tag"])
	}
}
