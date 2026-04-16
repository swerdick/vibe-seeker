package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

// TagPrevalence represents a tag and its global prevalence.
type TagPrevalence struct {
	Tag        string  `json:"tag"`
	Prevalence float64 `json:"prevalence"`
}

// TagRelation represents a tag related to a source tag by artist co-occurrence.
type TagRelation struct {
	Tag      string  `json:"tag"`
	Strength float64 `json:"strength"`
}

// GetTopTags returns the most prevalent tags across all cached artists,
// ranked by the number of distinct artists tagged with each.
func (s *ArtistTagStore) GetTopTags(ctx context.Context, limit int) ([]TagPrevalence, error) {
	rows, err := s.pool.Query(ctx, `
		WITH ranked AS (
			SELECT tag, COUNT(DISTINCT artist_name) AS artist_count
			FROM artist_tags
			GROUP BY tag
			ORDER BY artist_count DESC
			LIMIT $1
		)
		SELECT tag, artist_count::float8 / MAX(artist_count) OVER () AS prevalence
		FROM ranked
		ORDER BY prevalence DESC
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying top tags: %w", err)
	}
	defer rows.Close()

	var tags []TagPrevalence
	for rows.Next() {
		var t TagPrevalence
		if err := rows.Scan(&t.Tag, &t.Prevalence); err != nil {
			return nil, fmt.Errorf("scanning top tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// GetRelatedTags returns tags that co-occur with the given tag across artists,
// ranked by the number of shared artists.
func (s *ArtistTagStore) GetRelatedTags(ctx context.Context, tag string, limit int) ([]TagRelation, error) {
	rows, err := s.pool.Query(ctx, `
		WITH co AS (
			SELECT b.tag, COUNT(DISTINCT a.artist_name) AS shared
			FROM artist_tags a
			JOIN artist_tags b ON a.artist_name = b.artist_name AND a.tag != b.tag
			WHERE a.tag = $1
			GROUP BY b.tag
			ORDER BY shared DESC
			LIMIT $2
		)
		SELECT tag, shared::float8 / MAX(shared) OVER () AS strength
		FROM co
		ORDER BY strength DESC
	`, tag, limit)
	if err != nil {
		return nil, fmt.Errorf("querying related tags for %q: %w", tag, err)
	}
	defer rows.Close()

	var relations []TagRelation
	for rows.Next() {
		var r TagRelation
		if err := rows.Scan(&r.Tag, &r.Strength); err != nil {
			return nil, fmt.Errorf("scanning related tag: %w", err)
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// GetStaleArtistNames returns artist names whose cached tags are older than
// the given duration. Used by the background tag enrichment job.
func (s *ArtistTagStore) GetStaleArtistNames(ctx context.Context, olderThan time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-olderThan)
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT artist_name FROM artist_tags WHERE fetched_at < $1 ORDER BY artist_name`,
		cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("querying stale artist names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning stale artist name: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
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
		// Normalize casing to match Last.fm tag convention (lowercase).
		genre = strings.ToLower(strings.TrimSpace(genre))
		subGenre = strings.ToLower(strings.TrimSpace(subGenre))

		if genre != "" && !seen[genre] {
			tags = append(tags, lastfm.Tag{Name: genre, Count: 80})
			seen[genre] = true
		}
		if subGenre != "" && !seen[subGenre] {
			tags = append(tags, lastfm.Tag{Name: subGenre, Count: 60})
			seen[subGenre] = true
		}
	}
	return tags, rows.Err()
}
