import { useState, useMemo } from "react";
import { useAuthContext } from "../contexts/AuthContext";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeGraph from "../components/VibeGraph";
import { useVibeGraph } from "../hooks/useVibeGraph";
import { useVenueMatching } from "../hooks/useVenueMatching";
import { useVenues } from "../hooks/useVenues";
import { useTurnstile } from "../hooks/useTurnstile";
import type { VenueData } from "../types";

export default function Explore() {
  const auth = useAuthContext();
  const [selectedVenue, setSelectedVenue] = useState<VenueData | null>(null);
  const [minMatch, setMinMatch] = useState(0);

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
      <TopBar anonymous />
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
