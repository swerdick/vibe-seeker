import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import Map, { Marker, Popup } from "react-map-gl/maplibre";
import "maplibre-gl/dist/maplibre-gl.css";

interface User {
  spotify_id: string;
  display_name: string;
}

interface ShowSummary {
  name: string;
  date: string;
  price_min: number;
  price_max: number;
  url: string;
}

interface VenueData {
  ID: string;
  Name: string;
  Latitude: number;
  Longitude: number;
  Address: string;
  City: string;
  State: string;
  ShowsTracked: number;
  shows: ShowSummary[] | null;
}

const NYC_VIEW = {
  latitude: 40.7128,
  longitude: -74.006,
  zoom: 12,
};

const MAP_STYLE = "https://tiles.openfreemap.org/styles/liberty";

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
        setVenues(data.venues || []);
      })
      .catch(() => {
        setVenues([]);
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
          fetchVenues();
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
    <div className="app-layout">
      <div className="top-bar">
        <span className="top-bar-greeting">Hello, {user?.display_name}</span>
        <div className="top-bar-actions">
          <button className="button" onClick={handleSync} disabled={syncing}>
            {syncing ? "Syncing..." : "Sync Vibe"}
          </button>
          {vibeError && <span className="error">{vibeError}</span>}
          <button
            className="button"
            onClick={handleVenueSync}
            disabled={venuesSyncing}
          >
            {venuesSyncing ? "Syncing..." : "Sync Venues"}
          </button>
          {venueError && <span className="error">{venueError}</span>}
          {venueCount !== null && (
            <span className="venue-count">{venueCount} venues</span>
          )}
          <button className="button button-secondary" onClick={handleLogout}>
            Log out
          </button>
        </div>
      </div>
      <div className="main-content">
        <div className="map-container">
          <Map
            initialViewState={NYC_VIEW}
            style={{ width: "100%", height: "100%" }}
            mapStyle={MAP_STYLE}
          >
            {venues.map((venue) => (
              <Marker
                key={venue.ID}
                latitude={venue.Latitude}
                longitude={venue.Longitude}
                onClick={(e) => {
                  e.originalEvent.stopPropagation();
                  setSelectedVenue(venue);
                }}
              >
                <div
                  className={`venue-marker ${venue.ShowsTracked > 0 ? "venue-marker-active" : ""}`}
                />
              </Marker>
            ))}
            {selectedVenue && (
              <Popup
                latitude={selectedVenue.Latitude}
                longitude={selectedVenue.Longitude}
                onClose={() => setSelectedVenue(null)}
                closeOnClick={false}
                maxWidth="300px"
              >
                <div className="venue-popup">
                  <h3>{selectedVenue.Name}</h3>
                  {selectedVenue.Address && <p>{selectedVenue.Address}</p>}
                  <p className="venue-popup-meta">
                    {selectedVenue.ShowsTracked} shows tracked
                  </p>
                  {selectedVenue.shows && selectedVenue.shows.length > 0 && (
                    <div className="venue-popup-shows">
                      <h4>Upcoming Shows</h4>
                      <ul>
                        {selectedVenue.shows.slice(0, 5).map((show, i) => (
                          <li key={i}>
                            <span className="show-name">{show.name}</span>
                            <span className="show-date">
                              {new Date(show.date).toLocaleDateString()}
                            </span>
                            {show.price_min > 0 && (
                              <span className="show-price">
                                ${show.price_min}
                                {show.price_max > show.price_min &&
                                  `-$${show.price_max}`}
                              </span>
                            )}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              </Popup>
            )}
          </Map>
        </div>
        <div className="sidebar">
          <h2>Your Vibe</h2>
          {genres && Object.keys(genres).length > 0 ? (
            <div className="genre-list">
              <ul>
                {Object.entries(genres)
                  .sort(([, a], [, b]) => b - a)
                  .map(([genre, weight]) => (
                    <li key={genre}>
                      <span className="genre-name">{genre}</span>
                      <span className="genre-bar-track">
                        <span
                          className="genre-bar"
                          style={{
                            display: "block",
                            width: `${weight * 100}%`,
                          }}
                        />
                      </span>
                    </li>
                  ))}
              </ul>
            </div>
          ) : (
            <p className="sidebar-empty">
              Click "Sync Vibe" to load your music profile.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
