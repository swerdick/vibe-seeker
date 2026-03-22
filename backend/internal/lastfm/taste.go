package lastfm

import "strings"

// ArtistRanking represents an artist's position in the user's top artists
// for a given time range.
type ArtistRanking struct {
	Name            string
	Position        int     // 0-indexed rank position
	RangeMultiplier float32 // 1.0 for medium_term, 0.5 for short_term
}

// ComputeTagWeights aggregates Last.fm tag data across multiple artists,
// weighted by each artist's rank position and time range.
//
// For each artist at position i: base weight = (1.0 - i*0.02) × rangeMultiplier.
// For each tag on that artist: weight += baseWeight × (tag.Count / 100.0).
// Final weights are normalized so the maximum is 1.0.
func ComputeTagWeights(artistTags map[string][]Tag, rankings []ArtistRanking) map[string]float32 {
	weights := make(map[string]float32)

	for _, r := range rankings {
		tags, ok := artistTags[strings.ToLower(r.Name)]
		if !ok {
			continue
		}

		rankWeight := float32(1.0) - float32(r.Position)*0.02
		if rankWeight < 0 {
			rankWeight = 0
		}
		baseWeight := rankWeight * r.RangeMultiplier

		for _, t := range tags {
			weights[t.Name] += baseWeight * (float32(t.Count) / 100.0)
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
