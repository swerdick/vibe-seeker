import { useState, useMemo, useCallback } from "react";
import { useAuthContext } from "../contexts/AuthContext";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeGraph from "../components/VibeGraph";
import VibeSidebar from "../components/VibeSidebar";
import GuidedTour from "../components/GuidedTour";
import { shouldAutoStartTour, resetTourComplete } from "../utils/tour";
import { useVibeGraph } from "../hooks/useVibeGraph";
import { useVenueMatching } from "../hooks/useVenueMatching";
import { useVenues } from "../hooks/useVenues";
import { useTurnstile } from "../hooks/useTurnstile";
import type { VenueData } from "../types";

export default function Explore() {
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

  const turnstile = useTurnstile({
    enabled: !auth.authenticated && !auth.loading,
    onAuthenticated: auth.refresh,
  });

  const authenticated = auth.authenticated || turnstile.authenticated;

  const venues = useVenues(authenticated);
  const graph = useVibeGraph(null, authenticated);

  // Build a vibes record from graph nodes for matching.
  const vibesFromGraph = useMemo(() => {
    const vibes: Record<string, number> = {};
    for (const node of graph.nodes) {
      vibes[node.id] = node.prevalence;
    }
    return Object.keys(vibes).length > 0 ? vibes : null;
  }, [graph.nodes]);

  const { venueScores, visibleVenues } = useVenueMatching(
    vibesFromGraph,
    graph.selectedTags,
    venues.venues,
    minMatch,
  );

  // Show captcha screen before authenticated.
  if (!authenticated) {
    return (
      <div className="page">
        <h1>Vibe Seeker</h1>
        <p>Discover venues that match your vibe.</p>
        {(auth.loading || turnstile.loading) && <p>Verifying...</p>}
        {turnstile.error && <p className="error">{turnstile.error}</p>}
        <div id="turnstile-widget" />
      </div>
    );
  }

  return (
    <div className="app-layout">
      <GuidedTour run={tourRunning} onFinish={handleTourFinish} />
      <TopBar anonymous onStartTour={handleStartTour} />
      <div className="main-content">
        <VenueMap
          venues={visibleVenues}
          venueScores={venueScores}
          selectedVenue={selectedVenue}
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
          <div className={`vibe-list-overlay ${viewMode === "list" ? "open" : ""}`} aria-hidden={viewMode !== "list"}>
            <VibeSidebar
              genres={vibesFromGraph}
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
