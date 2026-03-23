package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
)

const ArtistTagTTL = 7 * 24 * time.Hour // 7 days

// ArtistTagStore provides read-through caching for Last.fm artist tags.
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
	if time.Since(fetchedAt) > ArtistTagTTL {
		if _, err := s.pool.Exec(ctx, `DELETE FROM artist_tags WHERE artist_name = $1`, artistName); err != nil {
			slog.Error("failed to delete stale artist tags", "artist", artistName, "error", err)
		}
		return nil, nil
	}

	return tags, nil
}

// UpsertArtistTags replaces all cached tags for an artist.
func (s *ArtistTagStore) UpsertArtistTags(ctx context.Context, artistName string, tags []lastfm.Tag) error {
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
			`INSERT INTO artist_tags (artist_name, tag, count, fetched_at) VALUES ($1, $2, $3, NOW())`,
			artistName, t.Name, t.Count,
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
