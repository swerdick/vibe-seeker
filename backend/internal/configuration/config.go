package configuration

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Environment         string
	LogLevel            string
	Port                int
	AppName             string
	CORSOrigin          string
	OtelEnabled         bool
	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRedirectURI  string
	JWTSecret           string
	FrontendURL         string
	DatabaseURL         string
	LastFMAPIKey        string
	SecureCookie        bool
}

func NewConfig() Config {
	return Config{
		Environment:         readEnvironmentVariable("ENVIRONMENT", "local"),
		LogLevel:            readEnvironmentVariable("LOG_LEVEL", "info"),
		Port:                readIntEnvironmentVariable("PORT", 8080),
		AppName:             readEnvironmentVariable("APP_NAME", "vibe-seeker-api"),
		CORSOrigin:          readEnvironmentVariable("CORS_ORIGIN", "http://localhost:5173"),
		OtelEnabled:         readEnvironmentVariable("OTEL_ENABLED", "false") == "true",
		SpotifyClientID:     readEnvironmentVariable("SPOTIFY_CLIENT_ID", ""),
		SpotifyClientSecret: readEnvironmentVariable("SPOTIFY_CLIENT_SECRET", ""),
		SpotifyRedirectURI:  readEnvironmentVariable("SPOTIFY_REDIRECT_URI", "http://localhost:8080/api/auth/callback"),
		JWTSecret:           readEnvironmentVariable("JWT_SECRET", ""),
		FrontendURL:         readEnvironmentVariable("FRONTEND_URL", "http://localhost:5173"),
		DatabaseURL:         readEnvironmentVariable("DATABASE_URL", "postgres://vibe_seeker:vibe_seeker@localhost:5432/vibe_seeker?sslmode=disable"),
		LastFMAPIKey:        readEnvironmentVariable("LASTFM_API_KEY", ""),
		SecureCookie:        readEnvironmentVariable("ENVIRONMENT", "local") != "local",
	}
}

func readEnvironmentVariable(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	if defaultValue == "" {
		slog.Warn("required environment variable not set", "key", key)
	}

	return defaultValue
}

func readIntEnvironmentVariable(key string, defaultValue int) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		slog.Error(fmt.Sprintf("environment variable %s is not a valid integer", key), "value", raw)
		return defaultValue
	}

	return value
}
