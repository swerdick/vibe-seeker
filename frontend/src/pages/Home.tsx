import { useState } from "react";
import { useAuthContext } from "../contexts/AuthContext";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeGraph from "../components/VibeGraph";
import type { VenueData } from "../types";
import { useVibeGraph } from "../hooks/useVibeGraph";
import { useVenueMatching } from "../hooks/useVenueMatching";
import { useVibes } from "../hooks/useVibes";
import { useVenues } from "../hooks/useVenues";
import { useSyncAction } from "../hooks/useSyncAction";

export default function Home() {
  const auth = useAuthContext();
  const [selectedVenue, setSelectedVenue] = useState<VenueData | null>(null);
  const [minMatch, setMinMatch] = useState(0);

  const vibes = useVibes(auth.authenticated);
  const venues = useVenues(auth.authenticated);

  // Clear selection if the selected venue is no longer in the list.
  const effectiveSelectedVenue =
    selectedVenue && venues.venues.some((v) => v.ID === selectedVenue.ID)
      ? selectedVenue
      : null;

  const graph = useVibeGraph(vibes.vibes);

  const { venueScores, visibleVenues } = useVenueMatching(
    vibes.vibes,
    graph.selectedTags,
    venues.venues,
    minMatch,
  );

  const vibeSync = useSyncAction<{ vibe_count: number }>(
    "/api/vibe/sync",
    { errorMessage: "Failed to sync vibe from Spotify.", refetch: vibes.refetch },
  );

  const venueSync = useSyncAction<{ venues_count: number }>(
    "/api/venues/sync",
    { errorMessage: "Failed to sync venues from Ticketmaster.", refetch: venues.refetch },
  );

  const venueVibeSync = useSyncAction<{ vibes_computed: number }>(
    "/api/venues/vibes",
    { errorMessage: "Failed to compute venue vibes.", refetch: venues.refetch },
  );

  return (
    <div className="app-layout">
      <TopBar
        displayName={auth.user?.display_name || ""}
        syncing={vibeSync.syncing}
        vibeError={vibeSync.error}
        venuesSyncing={venueSync.syncing}
        venueError={venueSync.error || venueVibeSync.error}
        venueCount={venueSync.result?.venues_count ?? null}
        vibesSyncing={venueVibeSync.syncing}
        vibesComputed={venueVibeSync.result?.vibes_computed ?? null}
        onSyncVibe={vibeSync.execute}
        onSyncVenues={venueSync.execute}
        onSyncVenueVibes={venueVibeSync.execute}
        onLogout={auth.logout}
      />
      <div className="main-content">
        <VenueMap
          venues={visibleVenues}
          venueScores={venueScores}
          selectedVenue={effectiveSelectedVenue}
          minMatch={minMatch}
          visibleCount={visibleVenues.length}
          onSelectVenue={setSelectedVenue}
          onClosePopup={() => setSelectedVenue(null)}
          onMinMatchChange={setMinMatch}
        />
        <VibeGraph
          nodes={graph.nodes}
          edges={graph.edges}
          newNodeIds={graph.newNodeIds}
          selectedCount={graph.selectedTags.size}
          totalCount={graph.nodes.length}
          onToggleNode={graph.toggleNode}
          onAddNode={graph.addNode}
          onReset={graph.reset}
          onSelectAll={graph.selectAll}
          onSelectNone={graph.selectNone}
        />
      </div>
    </div>
  );
}
