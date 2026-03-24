package vibes

import (
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

// VenueArtist represents an artist who played at a venue on a specific date.
type VenueArtist struct {
	ArtistName string
	ShowDate   time.Time
}

// ComputeVenueVibe builds a vibe vector for a venue from its show history.
// For each artist, the recency of their most recent show at the venue determines
// their weight. Tags are then accumulated across all artists and normalized.
func ComputeVenueVibe(artists []VenueArtist, artistTags map[string][]lastfm.Tag) map[string]float32 {
	// Group by artist, keeping the most recent show date (highest recency weight).
	bestWeight := make(map[string]float32)
	for _, a := range artists {
		name := strings.ToLower(a.ArtistName)
		w := RecencyWeight(a.ShowDate)
		if w > bestWeight[name] {
			bestWeight[name] = w
		}
	}

	// Build weighted artist list.
	weighted := make([]WeightedArtist, 0, len(bestWeight))
	for name, w := range bestWeight {
		weighted = append(weighted, WeightedArtist{Name: name, Weight: w})
	}

	return ComputeVibes(artistTags, weighted)
}
