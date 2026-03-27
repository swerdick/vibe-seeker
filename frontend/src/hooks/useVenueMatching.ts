import { useMemo } from "react";
import type { VenueData } from "../types";
import { cosineSimilarity } from "../utils/matching";

interface UseVenueMatchingResult {
  filteredVibes: Record<string, number>;
  venueScores: Map<string, number>;
  visibleVenues: VenueData[];
}

export function useVenueMatching(
  vibes: Record<string, number> | null,
  selectedTags: Set<string>,
  venues: VenueData[],
  minMatch: number,
): UseVenueMatchingResult {
  const filteredVibes = useMemo(() => {
    if (!vibes) return {};
    const filtered: Record<string, number> = {};
    for (const [tag, weight] of Object.entries(vibes)) {
      if (selectedTags.has(tag)) {
        filtered[tag] = weight;
      }
    }
    return filtered;
  }, [vibes, selectedTags]);

  const venueScores = useMemo(() => {
    const scores = new Map<string, number>();
    if (Object.keys(filteredVibes).length === 0) return scores;
    for (const venue of venues) {
      if (venue.vibes && Object.keys(venue.vibes).length > 0) {
        scores.set(venue.ID, cosineSimilarity(filteredVibes, venue.vibes));
      }
    }
    return scores;
  }, [venues, filteredVibes]);

  const visibleVenues = useMemo(() => {
    if (minMatch <= 0) return venues;
    return venues.filter((v) => (venueScores.get(v.ID) || 0) >= minMatch);
  }, [venues, venueScores, minMatch]);

  return { filteredVibes, venueScores, visibleVenues };
}
