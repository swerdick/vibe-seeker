package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VenueStore provides persistence operations for venues, shows, and artists.
type VenueStore struct {
	pool *pgxpool.Pool
}

func NewVenueStore(pool *pgxpool.Pool) (*VenueStore, error) {
	if pool == nil {
		return nil, errors.New("store: nil connection pool")
	}
	return &VenueStore{pool: pool}, nil
}

// Venue represents a venue record in the database.
type Venue struct {
	ID           string
	Name          string
	Latitude      float64
	Longitude     float64
	Address       string
	City          string
	State         string
	ImageURL      string
	BoxOfficeInfo []byte // JSONB
	ParkingDetail string
	GeneralInfo   []byte // JSONB
	Ada           []byte // JSONB
	ShowsTracked  int
	DataSource    string
	TMID          string
	FetchedAt     time.Time
}

// Show represents a show/event at a venue.
type Show struct {
	ID         string
	Name       string
	VenueID    string
	ShowDate   time.Time
	TicketURL  string
	PriceMin   float64
	PriceMax   float64
	Status     string
	DataSource string
}

// Artist represents a performer.
type Artist struct {
	ID       string
	Name     string
	ImageURL string
}

// ShowArtist links a show to an artist.
type ShowArtist struct {
	ShowID       string
	ArtistID     string
	BillingOrder int
}

// ShowClassification holds a Ticketmaster genre classification for a show.
type ShowClassification struct {
	ShowID   string
	Segment  string
	Genre    string
	SubGenre string
}

// Slugify converts an artist name to a URL-safe ID.
func Slugify(name string) string {
	lower := strings.ToLower(name)
	var b strings.Builder
	prevDash := false
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// UpsertVenues batch-upserts venues into the database.
func (s *VenueStore) UpsertVenues(ctx context.Context, venues []Venue) error {
	batch := &pgx.Batch{}
	for _, v := range venues {
		batch.Queue(`
			INSERT INTO venues (id, name, latitude, longitude, address, city, state, image_url,
				box_office_info, parking_detail, general_info, ada, data_source, tm_id, fetched_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				address = EXCLUDED.address,
				city = EXCLUDED.city,
				state = EXCLUDED.state,
				image_url = EXCLUDED.image_url,
				box_office_info = EXCLUDED.box_office_info,
				parking_detail = EXCLUDED.parking_detail,
				general_info = EXCLUDED.general_info,
				ada = EXCLUDED.ada,
				data_source = EXCLUDED.data_source,
				tm_id = EXCLUDED.tm_id,
				fetched_at = NOW()
		`, v.ID, v.Name, v.Latitude, v.Longitude, v.Address, v.City, v.State, v.ImageURL,
			v.BoxOfficeInfo, v.ParkingDetail, v.GeneralInfo, v.Ada, v.DataSource, v.TMID)
	}

	br := s.pool.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("upserting venues: %w", err)
	}
	return nil
}

// UpsertShows batch-upserts shows and increments shows_tracked on venues for new shows.
func (s *VenueStore) UpsertShows(ctx context.Context, shows []Show) error {
	batch := &pgx.Batch{}
	for _, sh := range shows {
		batch.Queue(`
			INSERT INTO shows (id, name, venue_id, show_date, ticket_url, price_min, price_max, status, data_source, fetched_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				venue_id = EXCLUDED.venue_id,
				show_date = EXCLUDED.show_date,
				ticket_url = EXCLUDED.ticket_url,
				price_min = EXCLUDED.price_min,
				price_max = EXCLUDED.price_max,
				status = EXCLUDED.status,
				data_source = EXCLUDED.data_source,
				fetched_at = NOW()
		`, sh.ID, sh.Name, sh.VenueID, sh.ShowDate, sh.TicketURL, sh.PriceMin, sh.PriceMax, sh.Status, sh.DataSource)
	}

	br := s.pool.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("upserting shows: %w", err)
	}

	// Update shows_tracked counts.
	_, err := s.pool.Exec(ctx, `
		UPDATE venues SET shows_tracked = sub.cnt
		FROM (SELECT venue_id, COUNT(*) AS cnt FROM shows GROUP BY venue_id) sub
		WHERE venues.id = sub.venue_id
	`)
	if err != nil {
		return fmt.Errorf("updating shows_tracked: %w", err)
	}

	return nil
}

// UpsertArtists batch-upserts artists by slugified name.
func (s *VenueStore) UpsertArtists(ctx context.Context, artists []Artist) error {
	batch := &pgx.Batch{}
	for _, a := range artists {
		batch.Queue(`
			INSERT INTO artists (id, name, image_url)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				image_url = COALESCE(EXCLUDED.image_url, artists.image_url)
		`, a.ID, a.Name, a.ImageURL)
	}

	br := s.pool.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("upserting artists: %w", err)
	}
	return nil
}

// UpsertShowArtists batch-inserts show-artist links, ignoring duplicates.
func (s *VenueStore) UpsertShowArtists(ctx context.Context, links []ShowArtist) error {
	batch := &pgx.Batch{}
	for _, l := range links {
		batch.Queue(`
			INSERT INTO show_artists (show_id, artist_id, billing_order)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING
		`, l.ShowID, l.ArtistID, l.BillingOrder)
	}

	br := s.pool.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("upserting show artists: %w", err)
	}
	return nil
}

// UpsertShowClassifications batch-inserts genre classifications for shows.
func (s *VenueStore) UpsertShowClassifications(ctx context.Context, classifications []ShowClassification) error {
	batch := &pgx.Batch{}
	for _, c := range classifications {
		batch.Queue(`
			INSERT INTO show_classifications (show_id, segment, genre, sub_genre)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, c.ShowID, c.Segment, c.Genre, c.SubGenre)
	}

	br := s.pool.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("upserting show classifications: %w", err)
	}
	return nil
}

// GetVenueFetchedAt returns the most recent fetched_at time for venues from
// the given data source. Returns nil if no venues exist for that source.
func (s *VenueStore) GetVenueFetchedAt(ctx context.Context, dataSource string) (*time.Time, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT MAX(fetched_at) FROM venues WHERE data_source = $1`, dataSource,
	).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("querying venue fetched_at: %w", err)
	}
	if t == nil || t.IsZero() {
		return nil, nil
	}
	return t, nil
}

// GetVenues returns all venues.
func (s *VenueStore) GetVenues(ctx context.Context) ([]Venue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, latitude, longitude, COALESCE(address, ''), COALESCE(city, ''), COALESCE(state, ''),
		       COALESCE(image_url, ''), box_office_info, COALESCE(parking_detail, ''),
		       general_info, ada,
		       shows_tracked, data_source, COALESCE(tm_id, ''), fetched_at
		FROM venues ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying venues: %w", err)
	}
	defer rows.Close()

	var venues []Venue
	for rows.Next() {
		var v Venue
		if err := rows.Scan(&v.ID, &v.Name, &v.Latitude, &v.Longitude, &v.Address,
			&v.City, &v.State, &v.ImageURL, &v.BoxOfficeInfo, &v.ParkingDetail,
			&v.GeneralInfo, &v.Ada,
			&v.ShowsTracked, &v.DataSource, &v.TMID, &v.FetchedAt); err != nil {
			return nil, fmt.Errorf("scanning venue: %w", err)
		}
		venues = append(venues, v)
	}
	return venues, rows.Err()
}

// ShowSummary holds minimal show data for venue popups.
type ShowSummary struct {
	Name     string    `json:"name"`
	Date     time.Time `json:"date"`
	PriceMin float64   `json:"price_min"`
	PriceMax float64   `json:"price_max"`
	URL      string    `json:"url"`
}

// GetShowsForVenues returns upcoming shows for multiple venues in a single query,
// keyed by venue ID. Returns up to 5 upcoming shows per venue.
func (s *VenueStore) GetShowsForVenues(ctx context.Context, venueIDs []string) (map[string][]ShowSummary, error) {
	if len(venueIDs) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT venue_id, name, show_date, COALESCE(price_min, 0), COALESCE(price_max, 0), COALESCE(ticket_url, '')
		FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY venue_id ORDER BY show_date ASC) AS rn
			FROM shows
			WHERE venue_id = ANY($1) AND show_date >= NOW()
		) sub
		WHERE rn <= 5
		ORDER BY venue_id, show_date ASC
	`, venueIDs)
	if err != nil {
		return nil, fmt.Errorf("querying shows for venues: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]ShowSummary)
	for rows.Next() {
		var venueID string
		var sh ShowSummary
		if err := rows.Scan(&venueID, &sh.Name, &sh.Date, &sh.PriceMin, &sh.PriceMax, &sh.URL); err != nil {
			return nil, fmt.Errorf("scanning show: %w", err)
		}
		result[venueID] = append(result[venueID], sh)
	}
	return result, rows.Err()
}

// VenueArtist represents an artist who played at a venue on a specific date.
type VenueArtist struct {
	ArtistName string
	ShowDate   time.Time
}

// GetVenueArtists returns all artists who played at a venue with their show dates.
func (s *VenueStore) GetVenueArtists(ctx context.Context, venueID string) ([]VenueArtist, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.name, s.show_date
		FROM artists a
		JOIN show_artists sa ON a.id = sa.artist_id
		JOIN shows s ON sa.show_id = s.id
		WHERE s.venue_id = $1
		ORDER BY s.show_date DESC
	`, venueID)
	if err != nil {
		return nil, fmt.Errorf("querying venue artists for %s: %w", venueID, err)
	}
	defer rows.Close()

	var result []VenueArtist
	for rows.Next() {
		var va VenueArtist
		if err := rows.Scan(&va.ArtistName, &va.ShowDate); err != nil {
			return nil, fmt.Errorf("scanning venue artist: %w", err)
		}
		result = append(result, va)
	}
	return result, rows.Err()
}

// GetAllVenueArtists returns artists + show dates for all venues in a single query,
// keyed by venue ID.
func (s *VenueStore) GetAllVenueArtists(ctx context.Context, venueIDs []string) (map[string][]VenueArtist, error) {
	if len(venueIDs) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT s.venue_id, a.name, s.show_date
		FROM artists a
		JOIN show_artists sa ON a.id = sa.artist_id
		JOIN shows s ON sa.show_id = s.id
		WHERE s.venue_id = ANY($1)
		ORDER BY s.venue_id, s.show_date DESC
	`, venueIDs)
	if err != nil {
		return nil, fmt.Errorf("querying all venue artists: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]VenueArtist)
	for rows.Next() {
		var venueID string
		var va VenueArtist
		if err := rows.Scan(&venueID, &va.ArtistName, &va.ShowDate); err != nil {
			return nil, fmt.Errorf("scanning venue artist: %w", err)
		}
		result[venueID] = append(result[venueID], va)
	}
	return result, rows.Err()
}

// UpsertVenueVibes replaces a venue's vibe profile atomically.
func (s *VenueStore) UpsertVenueVibes(ctx context.Context, venueID string, vibeWeights map[string]float32) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning venue vibe upsert tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM venue_vibes WHERE venue_id = $1`, venueID); err != nil {
		return fmt.Errorf("deleting old venue vibes for %s: %w", venueID, err)
	}

	batch := &pgx.Batch{}
	for tag, weight := range vibeWeights {
		batch.Queue(`INSERT INTO venue_vibes (venue_id, tag, weight) VALUES ($1, $2, $3)`,
			venueID, tag, float64(weight))
	}
	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("inserting venue vibes for %s: %w", venueID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing venue vibe upsert for %s: %w", venueID, err)
	}
	return nil
}

// GetVenueVibes retrieves a venue's vibe profile.
func (s *VenueStore) GetVenueVibes(ctx context.Context, venueID string) (map[string]float32, error) {
	rows, err := s.pool.Query(ctx, `SELECT tag, weight FROM venue_vibes WHERE venue_id = $1`, venueID)
	if err != nil {
		return nil, fmt.Errorf("querying venue vibes for %s: %w", venueID, err)
	}
	defer rows.Close()

	result := make(map[string]float32)
	for rows.Next() {
		var tag string
		var weight float64
		if err := rows.Scan(&tag, &weight); err != nil {
			return nil, fmt.Errorf("scanning venue vibe: %w", err)
		}
		result[tag] = float32(weight)
	}
	return result, rows.Err()
}

// GetAllVenueVibes retrieves vibe profiles for multiple venues in one query.
func (s *VenueStore) GetAllVenueVibes(ctx context.Context, venueIDs []string) (map[string]map[string]float32, error) {
	if len(venueIDs) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `SELECT venue_id, tag, weight FROM venue_vibes WHERE venue_id = ANY($1)`, venueIDs)
	if err != nil {
		return nil, fmt.Errorf("querying all venue vibes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]float32)
	for rows.Next() {
		var venueID, tag string
		var weight float64
		if err := rows.Scan(&venueID, &tag, &weight); err != nil {
			return nil, fmt.Errorf("scanning venue vibe: %w", err)
		}
		if result[venueID] == nil {
			result[venueID] = make(map[string]float32)
		}
		result[venueID][tag] = float32(weight)
	}
	return result, rows.Err()
}
