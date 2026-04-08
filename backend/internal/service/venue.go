package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
	"github.com/pseudo/vibe-seeker/backend/internal/vibes"
)

// TicketmasterClient provides the Ticketmaster methods needed for venue syncing.
type TicketmasterClient interface {
	SearchVenues(ctx context.Context, opts ticketmaster.VenueSearchOptions) ([]ticketmaster.Venue, error)
	SearchEvents(ctx context.Context, opts ticketmaster.EventSearchOptions) ([]ticketmaster.Event, error)
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

// nycSearchTiles covers the NYC metro area with overlapping 3-mile-radius
// circles, each staying under Ticketmaster's 1,000-result pagination limit.
var nycSearchTiles = []ticketmaster.VenueSearchOptions{
	{GeoPoint: "40.7128,-74.0060", Radius: "3"},  // Lower/Midtown Manhattan
	{GeoPoint: "40.8100,-73.9500", Radius: "3"},  // Upper Manhattan / Harlem
	{GeoPoint: "40.6782,-73.9442", Radius: "3"},  // Brooklyn
	{GeoPoint: "40.7282,-73.7949", Radius: "3"},  // Queens
	{GeoPoint: "40.8448,-73.8648", Radius: "3"},  // Bronx
	{GeoPoint: "40.7178,-74.0431", Radius: "3"},  // Hoboken / Jersey City
}

// VenueSyncResult contains the outcome of a venue sync operation.
type VenueSyncResult struct {
	Synced        bool
	Skipped       bool
	LastFetched   *time.Time
	VenueCount    int
	ShowCount     int
	VibesComputed int
}

// VenueWithDetails holds a venue plus its upcoming shows and vibe profile.
type VenueWithDetails struct {
	store.Venue
	Shows []store.ShowSummary    `json:"shows"`
	Vibes map[string]float32     `json:"vibes,omitempty"`
}

// VenueService orchestrates venue and event ingestion from Ticketmaster,
// venue vibe computation, and venue listing.
type VenueService struct {
	ticketmaster TicketmasterClient
	venues       VenueWriter
	venueReader  VenueReader
	venueVibes   VenueVibeWriter
	tagEnricher  *TagEnricher
}

// NewVenueService creates a VenueService.
func NewVenueService(tm TicketmasterClient, venues interface {
	VenueWriter
	VenueReader
	VenueVibeWriter
}, enricher *TagEnricher) (*VenueService, error) {
	if tm == nil {
		return nil, errors.New("venue service: nil ticketmaster client")
	}
	if venues == nil {
		return nil, errors.New("venue service: nil venue store")
	}
	if enricher == nil {
		return nil, errors.New("venue service: nil tag enricher")
	}
	return &VenueService{
		ticketmaster: tm,
		venues:       venues,
		venueReader:  venues,
		venueVibes:   venues,
		tagEnricher:  enricher,
	}, nil
}

// SyncVenues fetches NYC venues and upcoming events from Ticketmaster,
// persists them, and computes venue vibes.
func (s *VenueService) SyncVenues(ctx context.Context) (*VenueSyncResult, error) {
	log := observability.Logger(ctx)

	// TTL check: skip if data is fresh.
	lastFetched, err := s.venues.GetVenueFetchedAt(ctx, configuration.DataSourceTicketmaster)
	if err != nil {
		log.Error("failed to check venue TTL, proceeding with sync", "error", err)
	}
	if err == nil && lastFetched != nil && time.Since(*lastFetched) < configuration.VenueCacheTTL {
		return &VenueSyncResult{Skipped: true, LastFetched: lastFetched}, nil
	}

	syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configuration.VenueSyncTimeout)
	defer cancel()

	// 1. Fetch venues from tiled search regions, deduplicating by TM ID.
	seen := make(map[string]bool)
	var tmVenues []ticketmaster.Venue
	for _, tile := range nycSearchTiles {
		if syncCtx.Err() != nil {
			log.Error("venue fetch timed out", "fetched", len(tmVenues))
			break
		}
		tileVenues, err := s.ticketmaster.SearchVenues(syncCtx, tile)
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

	storeVenues := MapVenues(tmVenues)
	dbCtx := context.WithoutCancel(ctx)
	if err := s.venues.UpsertVenues(dbCtx, storeVenues); err != nil {
		return nil, fmt.Errorf("persisting venues: %w", err)
	}
	log.Info("venues synced", "count", len(storeVenues))

	// 2. Fetch events per venue with adaptive rate limiting.
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
			queue = nil
			continue
		case <-time.After(delay):
		}
		tmEvents, err := s.ticketmaster.SearchEvents(syncCtx, ticketmaster.EventSearchOptions{
			VenueID:       sv.TMID,
			StartDateTime: now,
		})
		if errors.Is(err, ticketmaster.ErrRateLimited) {
			queue = append(queue, sv)
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

	// 3. Persist everything.
	if len(shows) > 0 {
		if err := s.venues.UpsertShows(dbCtx, shows); err != nil {
			return nil, fmt.Errorf("persisting shows: %w", err)
		}
	}
	if len(artists) > 0 {
		if err := s.venues.UpsertArtists(dbCtx, artists); err != nil {
			return nil, fmt.Errorf("persisting artists: %w", err)
		}
	}
	if len(showArtists) > 0 {
		if err := s.venues.UpsertShowArtists(dbCtx, showArtists); err != nil {
			return nil, fmt.Errorf("persisting show-artist links: %w", err)
		}
	}
	if len(classifications) > 0 {
		if err := s.venues.UpsertShowClassifications(dbCtx, classifications); err != nil {
			return nil, fmt.Errorf("persisting show classifications: %w", err)
		}
	}

	log.Info("venue sync complete", "venues", len(storeVenues), "shows", len(shows), "artists", len(artists))

	// 4. Compute venue vibes from show artists + tags.
	vibesComputed := s.computeVenueVibes(syncCtx, ctx, storeVenues)

	return &VenueSyncResult{
		Synced:        true,
		VenueCount:    len(storeVenues),
		ShowCount:     len(shows),
		VibesComputed: vibesComputed,
	}, nil
}

// SyncVenueVibes recomputes vibe profiles for all venues without re-fetching
// from Ticketmaster.
func (s *VenueService) SyncVenueVibes(ctx context.Context) (int, error) {
	venues, err := s.venueReader.GetVenues(ctx)
	if err != nil {
		return 0, fmt.Errorf("reading venues: %w", err)
	}

	syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configuration.VenueSyncTimeout)
	defer cancel()

	computed := s.computeVenueVibes(syncCtx, ctx, venues)
	return computed, nil
}

// GetVenues returns all venues that have shows, with their upcoming shows
// and vibe profiles attached.
func (s *VenueService) GetVenues(ctx context.Context) ([]VenueWithDetails, error) {
	venues, err := s.venueReader.GetVenues(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading venues: %w", err)
	}

	log := observability.Logger(ctx)

	// Only include venues with shows.
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
		showsByVenue, err = s.venueReader.GetShowsForVenues(ctx, venueIDs)
		if err != nil {
			log.Error("failed to read shows for venues", "error", err)
		}
		vibesByVenue, err = s.venueReader.GetAllVenueVibes(ctx, venueIDs)
		if err != nil {
			log.Error("failed to read venue vibes", "error", err)
		}
	}

	var result []VenueWithDetails
	for _, id := range venueIDs {
		result = append(result, VenueWithDetails{
			Venue: venueMap[id],
			Shows: showsByVenue[id],
			Vibes: vibesByVenue[id],
		})
	}

	return result, nil
}

// computeVenueVibes computes vibe profiles for all venues with shows.
// Returns the number of venues with vibes computed.
func (s *VenueService) computeVenueVibes(syncCtx, parentCtx context.Context, venues []store.Venue) int {
	log := observability.Logger(parentCtx)

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
	allArtists, err := s.venueReader.GetAllVenueArtists(syncCtx, venueIDs)
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

	uniqueNames := make([]string, 0, len(uniqueArtists))
	for name := range uniqueArtists {
		uniqueNames = append(uniqueNames, name)
	}

	enrichResult := s.tagEnricher.Enrich(syncCtx, uniqueNames, configuration.LastFMRateLimit)

	// Compute vibes per venue.
	dbCtx := context.WithoutCancel(parentCtx)
	computed := 0
	for _, venueID := range venueIDs {
		venueArtists := allArtists[venueID]
		if len(venueArtists) == 0 {
			continue
		}

		va := make([]vibes.VenueArtist, len(venueArtists))
		for i, a := range venueArtists {
			va[i] = vibes.VenueArtist{ArtistName: a.ArtistName, ShowDate: a.ShowDate}
		}

		venueVibe := vibes.ComputeVenueVibe(va, enrichResult.ArtistTags)
		if len(venueVibe) == 0 {
			continue
		}

		if err := s.venueVibes.UpsertVenueVibes(dbCtx, venueID, venueVibe); err != nil {
			log.Error("failed to persist venue vibes", "venue", venueID, "error", err)
			continue
		}
		computed++
	}

	log.Info("venue vibes computed", "computed", computed, "total_venues", len(venueIDs))
	return computed
}

// MapVenues converts Ticketmaster venue data to store.Venue format,
// filtering out non-US venues and venues without coordinates.
func MapVenues(tmVenues []ticketmaster.Venue) []store.Venue {
	result := make([]store.Venue, 0, len(tmVenues))
	for _, v := range tmVenues {
		if v.Country.CountryCode != "" && v.Country.CountryCode != "US" {
			continue
		}
		lat := v.Lat()
		lng := v.Lng()
		if lat == 0 && lng == 0 {
			continue
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
