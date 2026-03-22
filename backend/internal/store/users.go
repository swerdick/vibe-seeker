package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserStore provides persistence operations for the users table.
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore creates a UserStore backed by the given connection pool.
// It returns an error if pool is nil.
func NewUserStore(pool *pgxpool.Pool) (*UserStore, error) {
	if pool == nil {
		return nil, errors.New("store: nil connection pool")
	}
	return &UserStore{pool: pool}, nil
}

// UpsertUser inserts a new user or updates the existing row on login.
// Tokens and display name are always refreshed; created_at is preserved.
func (s *UserStore) UpsertUser(ctx context.Context, id, displayName, accessToken, refreshToken string, tokenExpiry int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, display_name, access_token, refresh_token, token_expiry)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			display_name  = EXCLUDED.display_name,
			access_token  = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_expiry  = EXCLUDED.token_expiry,
			updated_at    = NOW()
	`, id, displayName, accessToken, refreshToken, tokenExpiry)
	if err != nil {
		return fmt.Errorf("upserting user %s: %w", id, err)
	}
	return nil
}

// UserTokens holds the token data needed to make Spotify API calls.
type UserTokens struct {
	AccessToken  string
	RefreshToken string
	TokenExpiry  int
}

// GetTokens retrieves the stored Spotify tokens for a user.
func (s *UserStore) GetTokens(ctx context.Context, userID string) (*UserTokens, error) {
	var t UserTokens
	err := s.pool.QueryRow(ctx,
		`SELECT access_token, refresh_token, token_expiry FROM users WHERE id = $1`,
		userID,
	).Scan(&t.AccessToken, &t.RefreshToken, &t.TokenExpiry)
	if err != nil {
		return nil, fmt.Errorf("getting tokens for user %s: %w", userID, err)
	}
	return &t, nil
}

// UpdateTokens refreshes the stored Spotify tokens for a user.
func (s *UserStore) UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, tokenExpiry int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET
			access_token  = $2,
			refresh_token = $3,
			token_expiry  = $4,
			updated_at    = NOW()
		WHERE id = $1
	`, userID, accessToken, refreshToken, tokenExpiry)
	if err != nil {
		return fmt.Errorf("updating tokens for user %s: %w", userID, err)
	}
	return nil
}

// UpsertGenres replaces a user's genre weights atomically.
// It deletes existing genres and inserts the new set within a transaction,
// then updates vibe_synced_at on the users row.
func (s *UserStore) UpsertGenres(ctx context.Context, userID string, genres map[string]float32) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning genre upsert tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM user_genres WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("deleting old genres for user %s: %w", userID, err)
	}

	batch := &pgx.Batch{}
	for genre, weight := range genres {
		batch.Queue(`INSERT INTO user_genres (user_id, genre, weight) VALUES ($1, $2, $3)`,
			userID, genre, weight)
	}
	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("inserting genres for user %s: %w", userID, err)
	}

	if _, err := tx.Exec(ctx, `UPDATE users SET vibe_synced_at = NOW(), updated_at = NOW() WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("updating vibe_synced_at for user %s: %w", userID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing genre upsert for user %s: %w", userID, err)
	}
	return nil
}

// GetGenres retrieves all genre weights for a user.
func (s *UserStore) GetGenres(ctx context.Context, userID string) (map[string]float32, error) {
	rows, err := s.pool.Query(ctx, `SELECT genre, weight FROM user_genres WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying genres for user %s: %w", userID, err)
	}
	defer rows.Close()

	genres := make(map[string]float32)
	for rows.Next() {
		var genre string
		var weight float32
		if err := rows.Scan(&genre, &weight); err != nil {
			return nil, fmt.Errorf("scanning genre row: %w", err)
		}
		genres[genre] = weight
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating genre rows: %w", err)
	}
	return genres, nil
}
