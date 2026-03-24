CREATE TABLE venue_vibes (
    venue_id TEXT NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    tag      TEXT NOT NULL,
    weight   DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (venue_id, tag)
);
