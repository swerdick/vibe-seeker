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
  onStartTour?: () => void;
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
  onStartTour,
}: TopBarProps) {
  if (anonymous) {
    return (
      <div className="top-bar">
        <span className="top-bar-greeting">Vibe Seeker</span>
        <div className="top-bar-actions">
          {onStartTour && (
            <button
              className="tour-help-btn"
              onClick={onStartTour}
              title="Show tutorial"
              aria-label="Show tutorial"
            >
              ?
            </button>
          )}
          <a href="/api/auth/login" className="button" title="Spotify login is currently limited to approved accounts" data-tour="sync-vibe">
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
        {onStartTour && (
          <button
            className="tour-help-btn"
            onClick={onStartTour}
            title="Show tutorial"
            aria-label="Show tutorial"
          >
            ?
          </button>
        )}
        <button className="button" onClick={onSyncVibe} disabled={syncing} data-tour="sync-vibe">
          {syncing ? "Syncing..." : "Sync Vibe"}
        </button>
        {vibeError && <span className="error">{vibeError}</span>}
        {import.meta.env.VITE_SHOW_SYNC_CONTROLS === "true" && (
          <>
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
