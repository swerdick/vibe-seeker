package webserver

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/handlers"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
)

// New builds the HTTP server with all routes and middleware wired up.
// Middleware chain (outermost first): otelhttp → CORS → mux
func New(cfg configuration.Config, pool *pgxpool.Pool) (*http.Server, error) {

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handlers.HealthCheck)

	userStore, err := store.NewUserStore(pool)
	if err != nil {
		return nil, fmt.Errorf("creating user store: %w", err)
	}

	spotifyClient := spotify.NewClient(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.SpotifyRedirectURI)
	authHandler, err := handlers.NewAuthHandler(spotifyClient, userStore, cfg.JWTSecret, cfg.FrontendURL, cfg.SecureCookie)
	if err != nil {
		return nil, fmt.Errorf("creating auth handler: %w", err)
	}

	mux.HandleFunc("GET /api/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/auth/callback", authHandler.Callback)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	// Protected routes — require a valid session cookie.
	requireAuth := middleware.RequireAuth(cfg.JWTSecret)
	mux.Handle("GET /api/auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))

	lastfmClient := lastfm.NewClient(cfg.LastFMAPIKey)
	vibeHandler, err := handlers.NewVibeHandler(spotifyClient, lastfmClient, userStore, userStore, userStore)
	if err != nil {
		return nil, fmt.Errorf("creating vibe handler: %w", err)
	}
	mux.Handle("POST /api/vibe/sync", requireAuth(http.HandlerFunc(vibeHandler.SyncVibe)))
	mux.Handle("GET /api/vibe", requireAuth(http.HandlerFunc(vibeHandler.GetVibe)))

	tmClient := ticketmaster.NewClient(cfg.TicketmasterAPIKey)
	venueStore, err := store.NewVenueStore(pool)
	if err != nil {
		return nil, fmt.Errorf("creating venue store: %w", err)
	}
	venueHandler, err := handlers.NewVenueHandler(tmClient, venueStore)
	if err != nil {
		return nil, fmt.Errorf("creating venue handler: %w", err)
	}
	mux.Handle("POST /api/venues/sync", requireAuth(http.HandlerFunc(venueHandler.SyncVenues)))
	mux.Handle("GET /api/venues", requireAuth(http.HandlerFunc(venueHandler.GetVenues)))

	var handler http.Handler = mux
	handler = middleware.CORS(cfg.CORSOrigin)(handler)
	handler = otelhttp.NewHandler(handler, cfg.AppName)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,
	}, nil
}

// ParseLogLevel converts a string log level to the corresponding slog.Level.
func ParseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
