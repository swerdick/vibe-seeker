CREATE TABLE user_genres (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    genre   TEXT NOT NULL,
    weight  REAL NOT NULL,
    PRIMARY KEY (user_id, genre)
);

CREATE INDEX idx_user_genres_genre ON user_genres (genre);
