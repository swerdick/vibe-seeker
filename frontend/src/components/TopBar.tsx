interface TopBarProps {
  anonymous?: boolean;
  displayName?: string;
  syncing?: boolean;
  vibeError?: string | null;
  venuesSyncing?: boolean;
  venueError?: string | null;
  venueCount?: number | null;
  vibesSyncing?: boolean;
  vibesComputed?: number | null;
  onSyncVibe?: () => void;
  onSyncVenues?: () => void;
  onSyncVenueVibes?: () => void;
  onLogout?: () => void;
}

export default function TopBar({
  anonymous,
  displayName,
  syncing,
  vibeError,
  venuesSyncing,
  venueError,
  venueCount,
  vibesSyncing,
  vibesComputed,
  onSyncVibe,
  onSyncVenues,
  onSyncVenueVibes,
  onLogout,
}: TopBarProps) {
  if (anonymous) {
    return (
      <div className="top-bar">
        <span className="top-bar-greeting">Vibe Seeker</span>
        <div className="top-bar-actions">
          <a href="/api/auth/login" className="button">
            Connect Spotify
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="top-bar">
      <span className="top-bar-greeting">Hello, {displayName}</span>
      <div className="top-bar-actions">
        {import.meta.env.VITE_SHOW_SYNC_CONTROLS !== "false" && (
          <>
            <button className="button" onClick={onSyncVibe} disabled={syncing}>
              {syncing ? "Syncing..." : "Sync Vibe"}
            </button>
            {vibeError && <span className="error">{vibeError}</span>}
            <button
              className="button"
              onClick={onSyncVenues}
              disabled={venuesSyncing}
            >
              {venuesSyncing ? "Syncing..." : "Sync Venues"}
            </button>
            {venueError && <span className="error">{venueError}</span>}
            {venueCount !== null && (
              <span className="venue-count">{venueCount} venues</span>
            )}
            <button
              className="button"
              onClick={onSyncVenueVibes}
              disabled={vibesSyncing}
            >
              {vibesSyncing ? "Computing..." : "Sync Venue Vibes"}
            </button>
            {vibesComputed !== null && (
              <span className="venue-count">{vibesComputed} venues updated</span>
            )}
          </>
        )}
        <button className="button button-secondary" onClick={onLogout}>
          Log out
        </button>
      </div>
    </div>
  );
}
