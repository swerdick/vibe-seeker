package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

// Tag sources.
const (
	TagSourceLastFM       = "lastfm"
	TagSourceTicketmaster = "ticketmaster"
)

// CachedTag extends lastfm.Tag with source information.
type CachedTag struct {
	lastfm.Tag
	Source string
}

// ArtistTagStore provides read-through caching for artist tags from multiple sources.
type ArtistTagStore struct {
	pool *pgxpool.Pool
}

func NewArtistTagStore(pool *pgxpool.Pool) (*ArtistTagStore, error) {
	if pool == nil {
		return nil, errors.New("store: nil connection pool")
	}
	return &ArtistTagStore{pool: pool}, nil
}

// GetCachedTags returns cached tags for an artist if within TTL.
// Returns nil (not an error) on cache miss or stale data.
// Stale entries are deleted automatically.
func (s *ArtistTagStore) GetCachedTags(ctx context.Context, artistName string) ([]lastfm.Tag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT tag, count, fetched_at FROM artist_tags WHERE artist_name = $1`,
		artistName,
	)
	if err != nil {
		return nil, fmt.Errorf("querying cached tags for %s: %w", artistName, err)
	}
	defer rows.Close()

	var tags []lastfm.Tag
	var fetchedAt time.Time
	for rows.Next() {
		var t lastfm.Tag
		if err := rows.Scan(&t.Name, &t.Count, &fetchedAt); err != nil {
			return nil, fmt.Errorf("scanning cached tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cached tags: %w", err)
	}

	if len(tags) == 0 {
		return nil, nil // cache miss
	}

	// Check TTL — if stale, delete and return miss.
	if time.Since(fetchedAt) > configuration.ArtistTagCacheTTL {
		if _, err := s.pool.Exec(ctx, `DELETE FROM artist_tags WHERE artist_name = $1`, artistName); err != nil {
			slog.Error("failed to delete stale artist tags", "artist", artistName, "error", err)
		}
		return nil, nil
	}

	return tags, nil
}

// GetCachedTagsWithSource returns cached tags with their source for weight differentiation.
func (s *ArtistTagStore) GetCachedTagsWithSource(ctx context.Context, artistName string) ([]CachedTag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT tag, count, source, fetched_at FROM artist_tags WHERE artist_name = $1`,
		artistName,
	)
	if err != nil {
		return nil, fmt.Errorf("querying cached tags for %s: %w", artistName, err)
	}
	defer rows.Close()

	var tags []CachedTag
	var fetchedAt time.Time
	for rows.Next() {
		var t CachedTag
		if err := rows.Scan(&t.Name, &t.Count, &t.Source, &fetchedAt); err != nil {
			return nil, fmt.Errorf("scanning cached tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating cached tags: %w", err)
	}

	if len(tags) == 0 {
		return nil, nil
	}

	if time.Since(fetchedAt) > configuration.ArtistTagCacheTTL {
		if _, err := s.pool.Exec(ctx, `DELETE FROM artist_tags WHERE artist_name = $1`, artistName); err != nil {
			slog.Error("failed to delete stale artist tags", "artist", artistName, "error", err)
		}
		return nil, nil
	}

	return tags, nil
}

// UpsertArtistTags replaces all cached tags for an artist with a given source.
func (s *ArtistTagStore) UpsertArtistTags(ctx context.Context, artistName string, tags []lastfm.Tag) error {
	return s.UpsertArtistTagsWithSource(ctx, artistName, tags, TagSourceLastFM)
}

// UpsertArtistTagsWithSource replaces all cached tags for an artist, recording the data source.
func (s *ArtistTagStore) UpsertArtistTagsWithSource(ctx context.Context, artistName string, tags []lastfm.Tag, source string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning artist tag upsert tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM artist_tags WHERE artist_name = $1`, artistName); err != nil {
		return fmt.Errorf("deleting old tags for %s: %w", artistName, err)
	}

	batch := &pgx.Batch{}
	for _, t := range tags {
		batch.Queue(
			`INSERT INTO artist_tags (artist_name, tag, count, source, fetched_at) VALUES ($1, $2, $3, $4, NOW())`,
			artistName, t.Name, t.Count, source,
		)
	}
	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("inserting tags for %s: %w", artistName, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing artist tag upsert for %s: %w", artistName, err)
	}
	return nil
}

// GetClassificationsForArtist returns Ticketmaster classifications for an artist
// by looking up their shows' classifications.
func (s *ArtistTagStore) GetClassificationsForArtist(ctx context.Context, artistName string) ([]lastfm.Tag, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT sc.genre, sc.sub_genre
		FROM show_classifications sc
		JOIN show_artists sa ON sc.show_id = sa.show_id
		JOIN artists a ON sa.artist_id = a.id
		WHERE LOWER(a.name) = LOWER($1) AND (sc.genre != '' OR sc.sub_genre != '')
	`, artistName)
	if err != nil {
		return nil, fmt.Errorf("querying TM classifications for %s: %w", artistName, err)
	}
	defer rows.Close()

	var tags []lastfm.Tag
	seen := make(map[string]bool)
	for rows.Next() {
		var genre, subGenre string
		if err := rows.Scan(&genre, &subGenre); err != nil {
			return nil, fmt.Errorf("scanning classification: %w", err)
		}
		// Add both genre and sub-genre as separate tags.
		if genre != "" && !seen[genre] {
			tags = append(tags, lastfm.Tag{Name: genre, Count: 80}) // broad genre
			seen[genre] = true
		}
		if subGenre != "" && !seen[subGenre] {
			tags = append(tags, lastfm.Tag{Name: subGenre, Count: 60}) // narrower sub-genre
			seen[subGenre] = true
		}
	}
	return tags, rows.Err()
}
