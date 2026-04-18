import { useState, useMemo, useCallback } from "react";
import { useAuthContext } from "../contexts/AuthContext";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeGraph from "../components/VibeGraph";
import VibeSidebar from "../components/VibeSidebar";
import GuidedTour from "../components/GuidedTour";
import { shouldAutoStartTour, resetTourComplete } from "../utils/tour";
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
  const [viewMode, setViewMode] = useState<"graph" | "list">("graph");
  const [tourRunning, setTourRunning] = useState(shouldAutoStartTour);

  const handleStartTour = useCallback(() => {
    resetTourComplete();
    setTourRunning(true);
  }, []);

  const handleTourFinish = useCallback(() => {
    setTourRunning(false);
  }, []);

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

  const vibeGenres = useMemo(() => {
    if (viewMode !== "list") return null;
    const genres: Record<string, number> = {};
    for (const node of graph.nodes) {
      genres[node.id] = node.prevalence;
    }
    return Object.keys(genres).length > 0 ? genres : null;
  }, [graph.nodes, viewMode]);

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
      <GuidedTour run={tourRunning} onFinish={handleTourFinish} />
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
        onStartTour={handleStartTour}
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
        <div className="vibe-panel" data-tour="vibe-panel">
          <div className="vibe-view-toggle">
            <button
              className={`vibe-toggle-btn ${viewMode === "graph" ? "active" : ""}`}
              onClick={() => setViewMode("graph")}
              title="Graph view"
              aria-label="Graph view"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <circle cx="4" cy="4" r="2" fill="currentColor" />
                <circle cx="12" cy="4" r="2" fill="currentColor" />
                <circle cx="8" cy="12" r="2" fill="currentColor" />
                <line x1="4" y1="4" x2="12" y2="4" stroke="currentColor" strokeWidth="1" />
                <line x1="4" y1="4" x2="8" y2="12" stroke="currentColor" strokeWidth="1" />
                <line x1="12" y1="4" x2="8" y2="12" stroke="currentColor" strokeWidth="1" />
              </svg>
            </button>
            <button
              className={`vibe-toggle-btn ${viewMode === "list" ? "active" : ""}`}
              onClick={() => setViewMode("list")}
              title="List view"
              aria-label="List view"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <rect x="1" y="2" width="14" height="2" rx="1" fill="currentColor" />
                <rect x="1" y="7" width="10" height="2" rx="1" fill="currentColor" />
                <rect x="1" y="12" width="12" height="2" rx="1" fill="currentColor" />
              </svg>
            </button>
          </div>
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
          <div className={`vibe-list-overlay ${viewMode === "list" ? "open" : ""}`} aria-hidden={viewMode !== "list"} inert={viewMode !== "list" ? true : undefined}>
            <VibeSidebar
              genres={vibeGenres}
              selectedGenres={graph.selectedTags}
              onToggleGenre={graph.toggleNode}
              onSelectAll={graph.selectAll}
              onSelectNone={graph.selectNone}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
