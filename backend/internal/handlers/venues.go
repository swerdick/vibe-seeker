package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/ratelimit"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
	"github.com/pseudo/vibe-seeker/backend/internal/vibes"
)

// nycSearchTiles covers the NYC metro area with overlapping 3-mile-radius
// circles, each staying under Ticketmaster's 1,000-result pagination limit.
// Results are deduplicated by venue ID after fetching.
var nycSearchTiles = []ticketmaster.VenueSearchOptions{
	{GeoPoint: "40.7128,-74.0060", Radius: "3"},  // Lower/Midtown Manhattan
	{GeoPoint: "40.8100,-73.9500", Radius: "3"},  // Upper Manhattan / Harlem
	{GeoPoint: "40.6782,-73.9442", Radius: "3"},  // Brooklyn
	{GeoPoint: "40.7282,-73.7949", Radius: "3"},  // Queens
	{GeoPoint: "40.8448,-73.8648", Radius: "3"},  // Bronx
	{GeoPoint: "40.7178,-74.0431", Radius: "3"},  // Hoboken / Jersey City
}

// VenueWriter persists venue, show, and artist data.
type VenueWriter interface {
	UpsertVenues(ctx context.Context, venues []store.Venue) error
	UpsertShows(ctx context.Context, shows []store.Show) error
	UpsertArtists(ctx context.Context, artists []store.Artist) error
	UpsertShowArtists(ctx context.Context, links []store.ShowArtist) error
	UpsertShowClassifications(ctx context.Context, classifications []store.ShowClassification) error
	GetVenueFetchedAt(ctx context.Context, dataSource string) (*time.Time, error)
}

// VenueReader retrieves venue data.
type VenueReader interface {
	GetVenues(ctx context.Context) ([]store.Venue, error)
	GetShowsForVenues(ctx context.Context, venueIDs []string) (map[string][]store.ShowSummary, error)
	GetAllVenueArtists(ctx context.Context, venueIDs []string) (map[string][]store.VenueArtist, error)
	GetAllVenueVibes(ctx context.Context, venueIDs []string) (map[string]map[string]float32, error)
}

// VenueVibeWriter persists venue vibe profiles.
type VenueVibeWriter interface {
	UpsertVenueVibes(ctx context.Context, venueID string, vibeWeights map[string]float32) error
}

// VenueHandler orchestrates venue and event ingestion from Ticketmaster.
type VenueHandler struct {
	Ticketmaster *ticketmaster.Client
	LastFM       *lastfm.Client
	Venues       VenueWriter
	VenueReader  VenueReader
	VenueVibes   VenueVibeWriter
	TagCache     TagCache
}

func NewVenueHandler(tm *ticketmaster.Client, lfm *lastfm.Client, venues interface {
	VenueWriter
	VenueReader
	VenueVibeWriter
}, tagCache TagCache) (*VenueHandler, error) {
	if tm == nil {
		return nil, errors.New("venues: nil ticketmaster client")
	}
	if lfm == nil {
		return nil, errors.New("venues: nil lastfm client")
	}
	if venues == nil {
		return nil, errors.New("venues: nil venue store")
	}
	if tagCache == nil {
		return nil, errors.New("venues: nil tag cache")
	}
	return &VenueHandler{
		Ticketmaster: tm,
		LastFM:       lfm,
		Venues:       venues,
		VenueReader:  venues,
		VenueVibes:   venues,
		TagCache:     tagCache,
	}, nil
}

// SyncVenues fetches NYC venues and upcoming events from Ticketmaster and
// persists them to the database.
func (h *VenueHandler) SyncVenues(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	log := observability.Logger(ctx)

	// TTL check: skip if data is fresh.
	lastFetched, err := h.Venues.GetVenueFetchedAt(ctx, configuration.DataSourceTicketmaster)
	if err != nil {
		log.Error("failed to check venue TTL, proceeding with sync", "error", err)
	}
	if err == nil && lastFetched != nil && time.Since(*lastFetched) < configuration.VenueCacheTTL {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"synced":       false,
			"reason":       "data is fresh",
			"last_fetched": lastFetched.Format(time.RFC3339),
		})
		return
	}

	syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configuration.VenueSyncTimeout)
	defer cancel()

	limiter := ratelimit.NewLimiter(configuration.TicketmasterRateLimit)
	defer limiter.Stop()

	// 1. Fetch venues from tiled search regions, deduplicating by TM ID.
	seen := make(map[string]bool)
	var tmVenues []ticketmaster.Venue
	for _, tile := range nycSearchTiles {
		if syncCtx.Err() != nil {
			log.Error("venue fetch timed out", "fetched", len(tmVenues))
			break
		}
		if err := limiter.Wait(syncCtx); err != nil {
			log.Error("rate limiter wait failed", "error", err)
			break
		}
		tileVenues, err := h.Ticketmaster.SearchVenues(syncCtx, tile)
		if err != nil {
			log.Error("failed to fetch ticketmaster venues", "tile", tile.GeoPoint, "error", err)
			continue
		}
		for _, v := range tileVenues {
			if !seen[v.ID] {
				seen[v.ID] = true
				tmVenues = append(tmVenues, v)
			}
		}
		log.Info("tile fetched", "center", tile.GeoPoint, "raw", len(tileVenues), "unique_total", len(tmVenues))
	}

	storeVenues := mapVenues(tmVenues)
	dbCtx := context.WithoutCancel(ctx)
	if err := h.Venues.UpsertVenues(dbCtx, storeVenues); err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to persist venues", "error", err)
		return
	}
	log.Info("venues synced", "count", len(storeVenues))

	// 2. Fetch events per venue with adaptive rate limiting.
	// On 429: requeue the venue and increase delay by 1s.
	// On 200: decrease delay by 1s (floor at configuration.TicketmasterRateLimit).
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	var shows []store.Show
	var artists []store.Artist
	var showArtists []store.ShowArtist
	var classifications []store.ShowClassification
	seenArtists := make(map[string]bool)

	queue := make([]store.Venue, len(storeVenues))
	copy(queue, storeVenues)
	delay := configuration.TicketmasterRateLimit
	processed := 0

	for len(queue) > 0 {
		if syncCtx.Err() != nil {
			log.Error("event fetch timed out", "processed", processed, "remaining", len(queue))
			break
		}

		sv := queue[0]
		queue = queue[1:]

		log.Info("fetching events", "venue", sv.Name, "processed", processed, "remaining", len(queue), "delay", delay)
		select {
		case <-syncCtx.Done():
			log.Error("event fetch timed out during delay", "processed", processed, "remaining", len(queue))
			queue = nil // break outer loop
			continue
		case <-time.After(delay):
		}
		tmEvents, err := h.Ticketmaster.SearchEvents(syncCtx, ticketmaster.EventSearchOptions{
			VenueID:       sv.TMID,
			StartDateTime: now,
		})
		if errors.Is(err, ticketmaster.ErrRateLimited) {
			queue = append(queue, sv) // requeue
			delay += 1 * time.Second
			log.Info("rate limited, backing off", "delay", delay, "remaining", len(queue))
			continue
		}
		if err != nil {
			log.Error("failed to fetch events for venue", "venue", sv.Name, "error", err)
			continue
		}

		// Successful request — reduce delay toward floor.
		if delay > configuration.TicketmasterRateLimit {
			delay -= 1 * time.Second
			if delay < configuration.TicketmasterRateLimit {
				delay = configuration.TicketmasterRateLimit
			}
		}
		processed++
		log.Info("events fetched", "venue", sv.Name, "events", len(tmEvents), "shows_total", len(shows)+len(tmEvents))

		for _, ev := range tmEvents {
			showID := configuration.IDPrefixTicketmaster + ev.ID
			showDate, err := time.Parse(time.RFC3339, ev.Dates.Start.DateTime)
			if err != nil {
				// Try localDate as fallback.
				showDate, err = time.Parse("2006-01-02", ev.Dates.Start.LocalDate)
				if err != nil {
					log.Error("skipping event with unparseable date", "event", ev.Name, "dateTime", ev.Dates.Start.DateTime)
					continue
				}
			}

			var priceMin, priceMax float64
			if len(ev.PriceRanges) > 0 {
				priceMin = ev.PriceRanges[0].Min
				priceMax = ev.PriceRanges[0].Max
			}

			shows = append(shows, store.Show{
				ID:         showID,
				Name:       ev.Name,
				VenueID:    sv.ID,
				ShowDate:   showDate,
				TicketURL:  ev.URL,
				PriceMin:   priceMin,
				PriceMax:   priceMax,
				Status:     ev.Dates.Status.Code,
				DataSource: configuration.DataSourceTicketmaster,
			})

			for i, attr := range ev.Embedded.Attractions {
				artistID := store.Slugify(attr.Name)
				if artistID == "" {
					continue
				}

				if !seenArtists[artistID] {
					var imgURL string
					if len(attr.Images) > 0 {
						imgURL = attr.Images[0].URL
					}
					artists = append(artists, store.Artist{
						ID:       artistID,
						Name:     attr.Name,
						ImageURL: imgURL,
					})
					seenArtists[artistID] = true
				}

				showArtists = append(showArtists, store.ShowArtist{
					ShowID:       showID,
					ArtistID:     artistID,
					BillingOrder: i + 1,
				})
			}

			for _, cl := range ev.Classifications {
				if cl.Segment.Name == "" && cl.Genre.Name == "" && cl.SubGenre.Name == "" {
					continue
				}
				classifications = append(classifications, store.ShowClassification{
					ShowID:   showID,
					Segment:  cl.Segment.Name,
					Genre:    cl.Genre.Name,
					SubGenre: cl.SubGenre.Name,
				})
			}
		}
	}

	// 4. Persist everything.
	if len(shows) > 0 {
		if err := h.Venues.UpsertShows(dbCtx, shows); err != nil {
			httpError(w, r, http.StatusInternalServerError, "internal error",
				"failed to persist shows", "error", err)
			return
		}
	}
	if len(artists) > 0 {
		if err := h.Venues.UpsertArtists(dbCtx, artists); err != nil {
			httpError(w, r, http.StatusInternalServerError, "internal error",
				"failed to persist artists", "error", err)
			return
		}
	}
	if len(showArtists) > 0 {
		if err := h.Venues.UpsertShowArtists(dbCtx, showArtists); err != nil {
			httpError(w, r, http.StatusInternalServerError, "internal error",
				"failed to persist show-artist links", "error", err)
			return
		}
	}
	if len(classifications) > 0 {
		if err := h.Venues.UpsertShowClassifications(dbCtx, classifications); err != nil {
			httpError(w, r, http.StatusInternalServerError, "internal error",
				"failed to persist show classifications", "error", err)
			return
		}
	}

	log.Info("venue sync complete", "venues", len(storeVenues), "shows", len(shows), "artists", len(artists))

	// 5. Compute venue vibes from show artists + Last.fm tags.
	vibesComputed := h.computeVenueVibes(syncCtx, ctx, storeVenues, log)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":          true,
		"venues_count":    len(storeVenues),
		"shows_count":     len(shows),
		"vibes_computed":  vibesComputed,
	})
}

// SyncVenueVibes computes vibe profiles for all venues without re-fetching
// from Ticketmaster. Useful for local dev when you want to recompute vibes
// without the full venue sync.
func (h *VenueHandler) SyncVenueVibes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	log := observability.Logger(ctx)

	venues, err := h.VenueReader.GetVenues(ctx)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to read venues for vibe computation", "error", err)
		return
	}

	syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configuration.VenueSyncTimeout)
	defer cancel()

	computed := h.computeVenueVibes(syncCtx, ctx, venues, log)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"synced":         true,
		"vibes_computed": computed,
	})
}

type venueResponse struct {
	store.Venue
	Shows []store.ShowSummary `json:"shows"`
	Vibes map[string]float32  `json:"vibes,omitempty"`
}

// GetVenues returns all cached venues with their upcoming shows.
func (h *VenueHandler) GetVenues(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	venues, err := h.VenueReader.GetVenues(ctx)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to read venues", "error", err)
		return
	}

	// Only include venues with shows to keep the response manageable.
	var venueIDs []string
	venueMap := make(map[string]store.Venue)
	for _, v := range venues {
		if v.ShowsTracked == 0 {
			continue
		}
		venueIDs = append(venueIDs, v.ID)
		venueMap[v.ID] = v
	}

	showsByVenue := make(map[string][]store.ShowSummary)
	vibesByVenue := make(map[string]map[string]float32)
	if len(venueIDs) > 0 {
		var err error
		showsByVenue, err = h.VenueReader.GetShowsForVenues(ctx, venueIDs)
		if err != nil {
			observability.Logger(ctx).Error("failed to read shows for venues", "error", err)
		}
		vibesByVenue, err = h.VenueReader.GetAllVenueVibes(ctx, venueIDs)
		if err != nil {
			observability.Logger(ctx).Error("failed to read venue vibes", "error", err)
		}
	}

	var result []venueResponse
	for _, id := range venueIDs {
		result = append(result, venueResponse{
			Venue: venueMap[id],
			Shows: showsByVenue[id],
			Vibes: vibesByVenue[id],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"venues": result,
		"count":  len(result),
	})
}

func mapVenues(tmVenues []ticketmaster.Venue) []store.Venue {
	result := make([]store.Venue, 0, len(tmVenues))
	for _, v := range tmVenues {
		if v.Country.CountryCode != "" && v.Country.CountryCode != "US" {
			continue // skip non-US venues (bad geocoding in TM data)
		}
		lat := v.Lat()
		lng := v.Lng()
		if lat == 0 && lng == 0 {
			continue // skip venues without coordinates
		}

		var imgURL string
		if len(v.Images) > 0 {
			imgURL = v.Images[0].URL
		}

		result = append(result, store.Venue{
			ID:            configuration.IDPrefixTicketmaster + v.ID,
			Name:          v.Name,
			Latitude:      lat,
			Longitude:     lng,
			Address:       v.Address.Line1,
			City:          v.City.Name,
			State:         v.State.StateCode,
			ImageURL:      imgURL,
			BoxOfficeInfo: v.BoxOfficeInfo,
			ParkingDetail: v.ParkingDetail,
			GeneralInfo:   v.GeneralInfo,
			Ada:           v.Ada,
			DataSource:    configuration.DataSourceTicketmaster,
			TMID:          v.ID,
		})
	}
	return result
}

// computeVenueVibes computes vibe profiles for all venues with shows.
// For each venue's artists, it fetches Last.fm tags (cached where possible)
// and computes a recency-weighted vibe vector.
// Returns the number of venues with vibes computed.
func (h *VenueHandler) computeVenueVibes(syncCtx, parentCtx context.Context, venues []store.Venue, log *slog.Logger) int {
	// Collect venues with shows.
	var venueIDs []string
	for _, v := range venues {
		if v.ShowsTracked > 0 {
			venueIDs = append(venueIDs, v.ID)
		}
	}
	if len(venueIDs) == 0 {
		return 0
	}

	// Get all artists for all venues in one query.
	allArtists, err := h.VenueReader.GetAllVenueArtists(syncCtx, venueIDs)
	if err != nil {
		log.Error("failed to get venue artists for vibe computation", "error", err)
		return 0
	}

	// Collect unique artist names across all venues.
	uniqueArtists := make(map[string]bool)
	for _, artists := range allArtists {
		for _, a := range artists {
			uniqueArtists[strings.ToLower(a.ArtistName)] = true
		}
	}

	// Fetch tags for all artists (cache-first, then Last.fm).
	limiter := ratelimit.NewLimiter(configuration.LastFMRateLimit)
	defer limiter.Stop()

	artistTags := make(map[string][]lastfm.Tag)
	cacheHits := 0
	for name := range uniqueArtists {
		if syncCtx.Err() != nil {
			log.Error("venue vibe tag fetch timed out", "fetched", len(artistTags), "total", len(uniqueArtists))
			break
		}

		cached, err := h.TagCache.GetCachedTags(syncCtx, name)
		if err != nil {
			log.Error("cache lookup failed", "artist", name, "error", err)
		}
		if cached != nil {
			artistTags[name] = cached
			cacheHits++
			continue
		}

		if err := limiter.Wait(syncCtx); err != nil {
			break
		}
		tags, err := h.LastFM.FetchArtistTags(syncCtx, name)
		if err != nil {
			log.Error("failed to fetch lastfm tags for venue vibe", "artist", name, "error", err)
			continue
		}

		if len(tags) > 0 {
			artistTags[name] = tags
			if cacheErr := h.TagCache.UpsertArtistTags(syncCtx, name, tags); cacheErr != nil {
				log.Error("failed to cache lastfm tags", "artist", name, "error", cacheErr)
			}
			continue
		}

		// Last.fm miss — fall back to Ticketmaster classifications.
		tmTags, tmErr := h.TagCache.GetClassificationsForArtist(syncCtx, name)
		if tmErr != nil {
			log.Error("failed to get TM classifications", "artist", name, "error", tmErr)
			continue
		}
		if len(tmTags) > 0 {
			log.Info("using ticketmaster classifications as fallback", "artist", name, "tags", len(tmTags))
			artistTags[name] = tmTags
			if cacheErr := h.TagCache.UpsertArtistTagsWithSource(syncCtx, name, tmTags, "ticketmaster"); cacheErr != nil {
				log.Error("failed to cache TM tags", "artist", name, "error", cacheErr)
			}
		} else {
			log.Info("no tag data available for artist", "artist", name, "sources", "lastfm,ticketmaster")
		}
	}
	log.Info("venue vibe tag fetch complete", "unique_artists", len(uniqueArtists), "cache_hits", cacheHits)

	// Compute vibes per venue.
	dbCtx := context.WithoutCancel(parentCtx)
	computed := 0
	for _, venueID := range venueIDs {
		venueArtists := allArtists[venueID]
		if len(venueArtists) == 0 {
			continue
		}

		// Convert store.VenueArtist to vibes.VenueArtist.
		va := make([]vibes.VenueArtist, len(venueArtists))
		for i, a := range venueArtists {
			va[i] = vibes.VenueArtist{ArtistName: a.ArtistName, ShowDate: a.ShowDate}
		}

		venueVibe := vibes.ComputeVenueVibe(va, artistTags)
		if len(venueVibe) == 0 {
			continue
		}

		if err := h.VenueVibes.UpsertVenueVibes(dbCtx, venueID, venueVibe); err != nil {
			log.Error("failed to persist venue vibes", "venue", venueID, "error", err)
			continue
		}
		computed++
	}

	log.Info("venue vibes computed", "computed", computed, "total_venues", len(venueIDs))
	return computed
}
