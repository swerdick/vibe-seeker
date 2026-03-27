import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeGraph from "../components/VibeGraph";
import type { User, VenueData } from "../types";
import { useVibeGraph } from "../hooks/useVibeGraph";
import { useVenueMatching } from "../hooks/useVenueMatching";

export default function Home() {
  const navigate = useNavigate();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [genres, setGenres] = useState<Record<string, number> | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [vibeError, setVibeError] = useState<string | null>(null);
  const [venuesSyncing, setVenuesSyncing] = useState(false);
  const [venueCount, setVenueCount] = useState<number | null>(null);
  const [venueError, setVenueError] = useState<string | null>(null);
  const [venues, setVenues] = useState<VenueData[]>([]);
  const [selectedVenue, setSelectedVenue] = useState<VenueData | null>(null);
  const [vibesSyncing, setVibesSyncing] = useState(false);
  const [vibesComputed, setVibesComputed] = useState<number | null>(null);
  const [minMatch, setMinMatch] = useState(0);

  // Graph initialized with user's Spotify vibes once loaded.
  const graph = useVibeGraph(genres);

  const { venueScores, visibleVenues } = useVenueMatching(
    genres,
    graph.selectedTags,
    venues,
    minMatch,
  );

  // --- Data fetching ---

  const fetchVibe = useCallback(() => {
    fetch("/api/vibe", { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("failed to load vibe");
        return res.json();
      })
      .then((data: { genres: Record<string, number>; genre_count: number }) => {
        setGenres(data.genres);
      })
      .catch(() => {
        setGenres(null);
      });
  }, []);

  const fetchVenues = useCallback(() => {
    fetch("/api/venues", { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("failed to load venues");
        return res.json();
      })
      .then((data: { venues: VenueData[]; count: number }) => {
        const nextVenues = data.venues || [];
        setVenues(nextVenues);
        setSelectedVenue((current) =>
          current && nextVenues.some((v) => v.ID === current.ID)
            ? current
            : null,
        );
      })
      .catch(() => {
        setVenues([]);
        setSelectedVenue(null);
      });
  }, []);

  useEffect(() => {
    fetch("/api/auth/me", { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("unauthorized");
        return res.json();
      })
      .then((data: User) => {
        setUser(data);
        setLoading(false);
      })
      .catch(() => {
        navigate("/", { replace: true });
      });
  }, [navigate]);

  useEffect(() => {
    if (user) {
      fetchVibe();
      fetchVenues();
    }
  }, [user, fetchVibe, fetchVenues]);

  // --- Sync handlers ---

  const handleSyncVibe = () => {
    setSyncing(true);
    setVibeError(null);
    fetch("/api/vibe/sync", { method: "POST", credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("sync failed");
        return res.json();
      })
      .then(() => fetchVibe())
      .catch(() => setVibeError("Failed to sync vibe from Spotify."))
      .finally(() => setSyncing(false));
  };

  const handleSyncVenues = () => {
    setVenuesSyncing(true);
    setVenueError(null);
    setVenueCount(null);
    fetch("/api/venues/sync", { method: "POST", credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("venue sync failed");
        return res.json();
      })
      .then((data: { venues_count?: number }) => {
        if (typeof data.venues_count === "number") setVenueCount(data.venues_count);
        fetchVenues();
      })
      .catch(() => setVenueError("Failed to sync venues from Ticketmaster."))
      .finally(() => setVenuesSyncing(false));
  };

  const handleSyncVenueVibes = () => {
    setVibesSyncing(true);
    setVibesComputed(null);
    fetch("/api/venues/vibes", { method: "POST", credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("venue vibe sync failed");
        return res.json();
      })
      .then((data: { vibes_computed?: number }) => {
        if (typeof data.vibes_computed === "number") setVibesComputed(data.vibes_computed);
        fetchVenues();
      })
      .catch(() => setVenueError("Failed to compute venue vibes."))
      .finally(() => setVibesSyncing(false));
  };

  const handleLogout = () => {
    fetch("/api/auth/logout", { method: "POST", credentials: "include" })
      .then(() => navigate("/", { replace: true }));
  };

  // --- Render ---

  if (loading) {
    return (
      <div className="page">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="app-layout">
      <TopBar
        displayName={user?.display_name || ""}
        syncing={syncing}
        vibeError={vibeError}
        venuesSyncing={venuesSyncing}
        venueError={venueError}
        venueCount={venueCount}
        vibesSyncing={vibesSyncing}
        vibesComputed={vibesComputed}
        onSyncVibe={handleSyncVibe}
        onSyncVenues={handleSyncVenues}
        onSyncVenueVibes={handleSyncVenueVibes}
        onLogout={handleLogout}
      />
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
