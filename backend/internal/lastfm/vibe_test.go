package lastfm

import (
	"math"
	"testing"
)

func TestComputeTagWeights_Basic(t *testing.T) {
	artistTags := map[string][]Tag{
		"artist one": {
			{Name: "rock", Count: 100},
			{Name: "indie", Count: 80},
		},
		"artist two": {
			{Name: "rock", Count: 90},
			{Name: "alternative", Count: 60},
		},
		"artist three": {
			{Name: "indie", Count: 100},
			{Name: "dream pop", Count: 70},
		},
	}

	rankings := []ArtistRanking{
		{Name: "Artist One", Position: 0, RangeMultiplier: 1.0},
		{Name: "Artist Two", Position: 1, RangeMultiplier: 1.0},
		{Name: "Artist Three", Position: 0, RangeMultiplier: 0.5},
	}

	weights := ComputeTagWeights(artistTags, rankings)

	// "rock" appears on two medium-term artists — should be the highest.
	if weights["rock"] != 1.0 {
		t.Errorf("rock weight = %f, want 1.0", weights["rock"])
	}

	for _, tag := range []string{"indie", "alternative", "dream pop"} {
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

func TestComputeTagWeights_Empty(t *testing.T) {
	weights := ComputeTagWeights(nil, nil)
	if len(weights) != 0 {
		t.Errorf("expected empty weights, got %d entries", len(weights))
	}
}

func TestComputeTagWeights_MissingArtist(t *testing.T) {
	// Artist in rankings but not in tag map — should be skipped.
	weights := ComputeTagWeights(
		map[string][]Tag{},
		[]ArtistRanking{{Name: "Unknown", Position: 0, RangeMultiplier: 1.0}},
	)
	if len(weights) != 0 {
		t.Errorf("expected empty weights, got %d entries", len(weights))
	}
}

func TestComputeTagWeights_Normalization(t *testing.T) {
	artistTags := map[string][]Tag{
		"a": {{Name: "x", Count: 100}, {Name: "y", Count: 50}},
		"b": {{Name: "x", Count: 80}},
	}
	rankings := []ArtistRanking{
		{Name: "A", Position: 0, RangeMultiplier: 1.0},
		{Name: "B", Position: 1, RangeMultiplier: 1.0},
	}

	weights := ComputeTagWeights(artistTags, rankings)

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

func TestComputeTagWeights_RankOrdering(t *testing.T) {
	artistTags := map[string][]Tag{
		"top":    {{Name: "top-tag", Count: 100}},
		"bottom": {{Name: "bottom-tag", Count: 100}},
	}
	rankings := []ArtistRanking{
		{Name: "top", Position: 0, RangeMultiplier: 1.0},
		{Name: "bottom", Position: 49, RangeMultiplier: 1.0},
	}

	weights := ComputeTagWeights(artistTags, rankings)
	if weights["top-tag"] <= weights["bottom-tag"] {
		t.Errorf("top-tag (%f) should have more weight than bottom-tag (%f)",
			weights["top-tag"], weights["bottom-tag"])
	}
}

func TestComputeTagWeights_CountInfluence(t *testing.T) {
	// Same rank, but different tag counts — higher count should produce higher weight.
	artistTags := map[string][]Tag{
		"artist": {
			{Name: "primary", Count: 100},
			{Name: "secondary", Count: 30},
		},
	}
	rankings := []ArtistRanking{
		{Name: "artist", Position: 0, RangeMultiplier: 1.0},
	}

	weights := ComputeTagWeights(artistTags, rankings)
	if weights["primary"] <= weights["secondary"] {
		t.Errorf("primary (%f) should have more weight than secondary (%f)",
			weights["primary"], weights["secondary"])
	}
}
