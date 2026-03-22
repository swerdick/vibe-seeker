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
