CREATE TABLE venues (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    latitude       DOUBLE PRECISION NOT NULL,
    longitude      DOUBLE PRECISION NOT NULL,
    address        TEXT,
    city           TEXT DEFAULT 'New York',
    state          TEXT DEFAULT 'NY',
    image_url       TEXT,
    box_office_info JSONB,
    parking_detail  TEXT,
    general_info    JSONB,
    ada             JSONB,
    shows_tracked   INTEGER DEFAULT 0,
    data_source     TEXT NOT NULL,
    tm_id           TEXT,
    fetched_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_venues_location ON venues(latitude, longitude);
CREATE INDEX idx_venues_data_source ON venues(data_source);
CREATE INDEX idx_venues_tm_id ON venues(tm_id);

CREATE TABLE shows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    venue_id    TEXT NOT NULL REFERENCES venues(id),
    show_date   TIMESTAMPTZ NOT NULL,
    ticket_url  TEXT,
    price_min   DOUBLE PRECISION,
    price_max   DOUBLE PRECISION,
    status      TEXT DEFAULT 'onsale',
    data_source TEXT NOT NULL DEFAULT 'ticketmaster',
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_shows_date ON shows(show_date);
CREATE INDEX idx_shows_venue ON shows(venue_id);

CREATE TABLE artists (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    image_url  TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_artists_name_lower ON artists(LOWER(name));

CREATE TABLE show_artists (
    show_id       TEXT NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    artist_id     TEXT NOT NULL REFERENCES artists(id),
    billing_order INTEGER DEFAULT 1,
    PRIMARY KEY (show_id, artist_id)
);

CREATE TABLE show_classifications (
    show_id   TEXT NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
    segment   TEXT NOT NULL DEFAULT '',
    genre     TEXT NOT NULL DEFAULT '',
    sub_genre TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (show_id, segment, genre, sub_genre)
);
