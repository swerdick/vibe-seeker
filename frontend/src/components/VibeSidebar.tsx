interface VibeSidebarProps {
  genres: Record<string, number> | null;
  selectedGenres: Set<string>;
  onToggleGenre: (genre: string) => void;
  onSelectAll: () => void;
  onSelectNone: () => void;
}

export default function VibeSidebar({
  genres,
  selectedGenres,
  onToggleGenre,
  onSelectAll,
  onSelectNone,
}: VibeSidebarProps) {
  return (
    <div className="sidebar">
      <h2>Your Vibe</h2>
      {genres && Object.keys(genres).length > 0 ? (
        <div className="genre-list">
          <div className="genre-controls">
            <button className="genre-control-btn" onClick={onSelectAll}>
              All
            </button>
            <button className="genre-control-btn" onClick={onSelectNone}>
              None
            </button>
            <span className="genre-count-label">
              {selectedGenres.size}/{Object.keys(genres).length}
            </span>
          </div>
          <ul>
            {Object.entries(genres)
              .sort(([, a], [, b]) => b - a)
              .map(([genre, weight]) => {
                const active = selectedGenres.has(genre);
                return (
                  <li
                    key={genre}
                    className={`genre-row ${active ? "" : "genre-row-disabled"}`}
                    role="checkbox"
                    aria-checked={active}
                    tabIndex={0}
                    onClick={() => onToggleGenre(genre)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        onToggleGenre(genre);
                      }
                    }}
                  >
                    <span className="genre-name">{genre}</span>
                    <span className="genre-bar-track">
                      <span
                        className="genre-bar"
                        style={{
                          display: "block",
                          width: `${weight * 100}%`,
                          opacity: active ? 1 : 0.3,
                        }}
                      />
                    </span>
                  </li>
                );
              })}
          </ul>
        </div>
      ) : (
        <p className="sidebar-empty">
          Click &quot;Sync Vibe&quot; to load your music profile.
        </p>
      )}
    </div>
  );
}
