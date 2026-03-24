import { useEffect, useState, useCallback, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import TopBar from "../components/TopBar";
import VenueMap from "../components/VenueMap";
import VibeSidebar from "../components/VibeSidebar";
import type { User, VenueData } from "../types";
import { cosineSimilarity } from "../utils/matching";

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
  const [selectedGenres, setSelectedGenres] = useState<Set<string>>(
    new Set(),
  );
  const [minMatch, setMinMatch] = useState(0);

  // Build filtered user vibe vector from selected genres.
  const filteredVibes = useMemo(() => {
    if (!genres) return {};
    const filtered: Record<string, number> = {};
    for (const [genre, weight] of Object.entries(genres)) {
      if (selectedGenres.has(genre)) {
        filtered[genre] = weight;
      }
    }
    return filtered;
  }, [genres, selectedGenres]);

  // Compute match scores for all venues.
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

  // Filter venues by minimum match threshold.
  const visibleVenues = useMemo(() => {
    if (minMatch <= 0) return venues;
    return venues.filter((v) => (venueScores.get(v.ID) || 0) >= minMatch);
  }, [venues, venueScores, minMatch]);

  // --- Data fetching ---

  const fetchVibe = useCallback(() => {
    fetch("/api/vibe", { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("failed to load vibe");
        return res.json();
      })
      .then((data: { genres: Record<string, number>; genre_count: number }) => {
        setGenres(data.genres);
        if (data.genres) {
          setSelectedGenres(new Set(Object.keys(data.genres)));
        }
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

  // --- Genre selection ---

  const toggleGenre = (genre: string) => {
    setSelectedGenres((prev) => {
      const next = new Set(prev);
      if (next.has(genre)) next.delete(genre);
      else next.add(genre);
      return next;
    });
  };

  const selectAllGenres = () => {
    if (genres) setSelectedGenres(new Set(Object.keys(genres)));
  };

  const selectNoGenres = () => {
    setSelectedGenres(new Set());
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
        <VibeSidebar
          genres={genres}
          selectedGenres={selectedGenres}
          onToggleGenre={toggleGenre}
          onSelectAll={selectAllGenres}
          onSelectNone={selectNoGenres}
        />
      </div>
    </div>
  );
}
