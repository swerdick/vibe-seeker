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
  const [tasteError, setTasteError] = useState<string | null>(null);

  const fetchTaste = useCallback(() => {
    fetch("/api/taste", { credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("failed to load taste");
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
      fetchTaste();
    }
  }, [user, fetchTaste]);

  const handleSync = () => {
    setSyncing(true);
    setTasteError(null);
    fetch("/api/taste/sync", { method: "POST", credentials: "include" })
      .then((res) => {
        if (!res.ok) throw new Error("sync failed");
        return res.json();
      })
      .then(() => {
        fetchTaste();
      })
      .catch(() => {
        setTasteError("Failed to sync taste from Spotify.");
      })
      .finally(() => {
        setSyncing(false);
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
        {syncing ? "Syncing..." : "Sync Taste"}
      </button>
      {tasteError && <p className="error">{tasteError}</p>}
      {genres && Object.keys(genres).length > 0 && (
        <div className="genre-list">
          <h2>Your Top Genres</h2>
          <ul>
            {Object.entries(genres)
              .sort(([, a], [, b]) => b - a)
              .map(([genre, weight]) => (
                <li key={genre}>
                  <span className="genre-name">{genre}</span>
                  <span
                    className="genre-bar"
                    style={{ width: `${weight * 100}%` }}
                  />
                </li>
              ))}
          </ul>
        </div>
      )}
      <button onClick={handleLogout}>Log out</button>
    </div>
  );
}
