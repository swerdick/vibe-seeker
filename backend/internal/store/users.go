package store

import (
	"context"
	"errors"
	"fmt"

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
