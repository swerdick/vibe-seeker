CREATE TABLE artist_tags (
    artist_name TEXT NOT NULL,
    tag         TEXT NOT NULL,
    count       INTEGER NOT NULL,
    fetched_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (artist_name, tag)
);
