package webserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/pseudo/vibe-seeker/backend/internal/app"
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/handlers"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
)

// New builds the HTTP server with all routes and middleware wired up.
// Middleware chain (outermost first): otelhttp → CORS → mux
func New(cfg configuration.Config, pool *pgxpool.Pool) (*http.Server, error) {

	svc, err := app.New(cfg, pool)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handlers.HealthCheck)

	// --- Handlers ---
	authHandler, err := handlers.NewAuthHandler(svc.SpotifyClient, svc.AuthSvc, cfg.FrontendURL, cfg.SecureCookie)
	if err != nil {
		return nil, fmt.Errorf("creating auth handler: %w", err)
	}

	mux.HandleFunc("GET /api/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/auth/callback", authHandler.Callback)
	mux.HandleFunc("POST /api/auth/anonymous", authHandler.AnonymousLogin)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	// Protected routes — require a valid session cookie.
	requireAuth := middleware.RequireAuth(cfg.JWTSecret)
	mux.Handle("GET /api/auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))

	vibeHandler, err := handlers.NewVibeHandler(svc.VibeSvc)
	if err != nil {
		return nil, fmt.Errorf("creating vibe handler: %w", err)
	}
	mux.Handle("POST /api/vibe/sync", requireAuth(http.HandlerFunc(vibeHandler.SyncVibe)))
	mux.Handle("GET /api/vibe", requireAuth(http.HandlerFunc(vibeHandler.GetVibe)))

	venueHandler, err := handlers.NewVenueHandler(svc.VenueSvc)
	if err != nil {
		return nil, fmt.Errorf("creating venue handler: %w", err)
	}
	mux.Handle("POST /api/venues/sync", requireAuth(http.HandlerFunc(venueHandler.SyncVenues)))
	mux.Handle("POST /api/venues/vibes", requireAuth(http.HandlerFunc(venueHandler.SyncVenueVibes)))
	mux.Handle("GET /api/venues", requireAuth(http.HandlerFunc(venueHandler.GetVenues)))

	exploreHandler, err := handlers.NewExploreHandler(svc.ExploreSvc)
	if err != nil {
		return nil, fmt.Errorf("creating explore handler: %w", err)
	}
	mux.Handle("GET /api/vibes/top", requireAuth(http.HandlerFunc(exploreHandler.GetTopVibes)))
	mux.Handle("GET /api/vibes/related", requireAuth(http.HandlerFunc(exploreHandler.GetRelatedVibes)))

	otlpRelay := handlers.NewOTLPRelayHandler(
		os.Getenv("OTEL_RELAY_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"),
		cfg.OtelEnabled,
	)
	mux.Handle("POST /api/otlp/v1/traces", requireAuth(http.HandlerFunc(otlpRelay.RelayTraces)))

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
