import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";

interface User {
  spotify_id: string;
  display_name: string;
}

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
    }
  }, [user, fetchVibe]);

  const handleSync = () => {
    setSyncing(true);
    setVibeError(null);
    fetch("/api/vibe/sync", { method: "POST", credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("sync failed");
        return res.json();
      })
      .then(() => {
        fetchVibe();
      })
      .catch(() => {
        setVibeError("Failed to sync vibe from Spotify.");
      })
      .finally(() => {
        setSyncing(false);
      });
  };

  const handleVenueSync = () => {
    setVenuesSyncing(true);
    setVenueError(null);
    setVenueCount(null);
    fetch("/api/venues/sync", { method: "POST", credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("venue sync failed");
        return res.json();
      })
      .then(
        (data: {
          venues_count?: number;
          count?: number;
          synced?: boolean;
        }) => {
          if (typeof data.venues_count === "number") {
            setVenueCount(data.venues_count);
          }
        },
      )
      .catch(() => {
        setVenueError("Failed to sync venues from Ticketmaster.");
      })
      .finally(() => {
        setVenuesSyncing(false);
      });
  };

  const handleLogout = () => {
    fetch("/api/auth/logout", {
      method: "POST",
      credentials: "include",
    }).then(() => {
      navigate("/", { replace: true });
    });
  };

  if (loading) {
    return (
      <div className="page">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="page">
      <h1>Hello, {user?.display_name}</h1>
      <p>You are logged in with Spotify.</p>
      <button className="button" onClick={handleSync} disabled={syncing}>
        {syncing ? "Syncing..." : "Sync Vibe"}
      </button>
      {vibeError && <p className="error">{vibeError}</p>}
      {genres && Object.keys(genres).length > 0 && (
        <div className="genre-list">
          <h2>Your Top Genres</h2>
          <ul>
            {Object.entries(genres)
              .sort(([, a], [, b]) => b - a)
              .map(([genre, weight]) => (
                <li key={genre}>
                  <span className="genre-name">{genre}</span>
                  <span className="genre-bar-track">
                    <span
                      className="genre-bar"
                      style={{ display: "block", width: `${weight * 100}%` }}
                    />
                  </span>
                </li>
              ))}
          </ul>
        </div>
      )}
      <button
        className="button"
        onClick={handleVenueSync}
        disabled={venuesSyncing}
      >
        {venuesSyncing ? "Syncing..." : "Sync Venues"}
      </button>
      {venueError && <p className="error">{venueError}</p>}
      {venueCount !== null && (
        <p>{venueCount} venues loaded from Ticketmaster</p>
      )}
      <button onClick={handleLogout}>Log out</button>
    </div>
  );
}
