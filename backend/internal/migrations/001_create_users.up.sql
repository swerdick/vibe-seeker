CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL,
    email           TEXT,
    avatar_url      TEXT,
    access_token    TEXT NOT NULL,
    refresh_token   TEXT NOT NULL,
    token_expiry    INTEGER NOT NULL,
    taste_synced_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
