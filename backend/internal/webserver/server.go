package webserver

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/handlers"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
)

// New builds the HTTP server with all routes and middleware wired up.
// Middleware chain (outermost first): otelhttp → CORS → mux
func New(cfg configuration.Config, pool *pgxpool.Pool) *http.Server {

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handlers.HealthCheck)

	userStore := store.NewUserStore(pool)
	spotify := auth.NewSpotifyClient(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.SpotifyRedirectURI)
	authHandler := handlers.NewAuthHandler(spotify, userStore, cfg.JWTSecret, cfg.FrontendURL, cfg.SecureCookie)

	mux.HandleFunc("GET /api/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/auth/callback", authHandler.Callback)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

	// Protected routes — require a valid session cookie.
	requireAuth := middleware.RequireAuth(cfg.JWTSecret)
	mux.Handle("GET /api/auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))

	var handler http.Handler = mux
	handler = middleware.CORS(cfg.CORSOrigin)(handler)
	handler = otelhttp.NewHandler(handler, cfg.AppName)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,
	}
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
