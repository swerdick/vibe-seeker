package vibes

import (
	"strings"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

// WeightedArtist represents an artist with a pre-computed weight.
// For user vibes, the weight comes from Spotify rank × time range multiplier.
// For venue vibes, the weight comes from show recency.
type WeightedArtist struct {
	Name   string
	Weight float32
}

// ComputeVibes aggregates Last.fm tags across weighted artists and normalizes
// so the maximum weight is 1.0.
//
// For each artist: contribution per tag = artist.Weight × (tag.Count / 100.0).
// Tags accumulate across all artists, then normalize.
func ComputeVibes(artistTags map[string][]lastfm.Tag, artists []WeightedArtist) map[string]float32 {
	weights := make(map[string]float32)

	for _, a := range artists {
		tags, ok := artistTags[strings.ToLower(a.Name)]
		if !ok {
			continue
		}
		for _, t := range tags {
			weights[t.Name] += a.Weight * (float32(t.Count) / 100.0)
		}
	}

	var max float32
	for _, w := range weights {
		if w > max {
			max = w
		}
	}
	if max > 0 {
		for tag := range weights {
			weights[tag] /= max
		}
	}

	return weights
}
