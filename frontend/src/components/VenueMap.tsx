import MapGL, { Marker, Popup } from "react-map-gl/maplibre";
import "maplibre-gl/dist/maplibre-gl.css";
import type { VenueData } from "../types";
import { getMatchColor } from "../utils/matching";

const NYC_VIEW = {
  latitude: 40.7128,
  longitude: -74.006,
  zoom: 12,
};

const MAP_STYLE = "https://tiles.openfreemap.org/styles/liberty";

interface VenueMapProps {
  venues: VenueData[];
  venueScores: Map<string, number>;
  selectedVenue: VenueData | null;
  minMatch: number;
  visibleCount: number;
  onSelectVenue: (venue: VenueData) => void;
  onClosePopup: () => void;
  onMinMatchChange: (value: number) => void;
}

export default function VenueMap({
  venues,
  venueScores,
  selectedVenue,
  minMatch,
  visibleCount,
  onSelectVenue,
  onClosePopup,
  onMinMatchChange,
}: VenueMapProps) {
  return (
    <div className="map-area">
      <div className="map-controls">
        <label className="match-slider-label">
          Min match: {Math.round(minMatch * 100)}%
          <input
            type="range"
            min="0"
            max="100"
            value={minMatch * 100}
            onChange={(e) => onMinMatchChange(Number(e.target.value) / 100)}
            className="match-slider"
          />
        </label>
        <span className="venue-count">{visibleCount} venues shown</span>
      </div>
      <div className="map-container">
        <MapGL
          initialViewState={NYC_VIEW}
          style={{ width: "100%", height: "100%" }}
          mapStyle={MAP_STYLE}
        >
          {venues.map((venue) => {
            const score = venueScores.get(venue.ID) || 0;
            return (
              <Marker
                key={venue.ID}
                latitude={venue.Latitude}
                longitude={venue.Longitude}
                onClick={(e) => {
                  e.originalEvent.stopPropagation();
                  onSelectVenue(venue);
                }}
              >
                <div
                  className="venue-marker"
                  style={{
                    backgroundColor: getMatchColor(score),
                    borderColor: getMatchColor(score),
                  }}
                  title={`${venue.Name} — ${Math.round(score * 100)}% match`}
                />
              </Marker>
            );
          })}
          {selectedVenue && (
            <Popup
              latitude={selectedVenue.Latitude}
              longitude={selectedVenue.Longitude}
              onClose={onClosePopup}
              closeOnClick={false}
              maxWidth="300px"
            >
              <VenuePopup
                venue={selectedVenue}
                score={venueScores.get(selectedVenue.ID) || 0}
              />
            </Popup>
          )}
        </MapGL>
      </div>
    </div>
  );
}

function VenuePopup({
  venue,
  score,
}: {
  venue: VenueData;
  score: number;
}) {
  return (
    <div className="venue-popup">
      <h3>{venue.Name}</h3>
      {venue.Address && <p>{venue.Address}</p>}
      {score > 0 && (
        <p className="venue-popup-match" style={{ color: getMatchColor(score) }}>
          {Math.round(score * 100)}% vibe match
        </p>
      )}
      {venue.vibes && Object.keys(venue.vibes).length > 0 && (
        <div className="venue-vibe-tags">
          {Object.entries(venue.vibes)
            .sort(([, a], [, b]) => b - a)
            .slice(0, 15)
            .map(([tag]) => (
              <span key={tag} className="vibe-tag">
                {tag}
              </span>
            ))}
        </div>
      )}
      <p className="venue-popup-meta">{venue.ShowsTracked} shows tracked</p>
      {venue.shows && venue.shows.length > 0 && (
        <div className="venue-popup-shows">
          <h4>Upcoming Shows</h4>
          <ul>
            {venue.shows.slice(0, 5).map((show, i) => (
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
  );
}
