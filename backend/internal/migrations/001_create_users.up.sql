CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL,
    access_token    TEXT NOT NULL, -- TODO: encrypt tokens at rest (app-level AES-GCM or pgcrypto)
    refresh_token   TEXT NOT NULL, -- TODO: encrypt tokens at rest
    token_expiry    INTEGER NOT NULL,
    vibe_synced_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
