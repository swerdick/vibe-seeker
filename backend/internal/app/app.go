// Package app provides shared service construction used by both the HTTP
// server and background Lambda handlers.
package app

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/lastfm"
	"github.com/pseudo/vibe-seeker/backend/internal/service"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
	"github.com/pseudo/vibe-seeker/backend/internal/store"
	"github.com/pseudo/vibe-seeker/backend/internal/ticketmaster"
)

// Services holds all service-layer dependencies, shared between the HTTP
// server and background Lambda handlers.
type Services struct {
	VenueSvc    *service.VenueService
	VibeSvc     *service.VibeService
	TagEnricher *service.TagEnricher
	AuthSvc     *service.AuthService
	ExploreSvc  *service.ExploreService

	// Stores exposed for handler construction.
	UserStore      *store.UserStore
	ArtistTagStore *store.ArtistTagStore
	VenueStore     *store.VenueStore

	// Clients exposed for handler construction.
	SpotifyClient *spotify.Client
}

// New builds all services from config and a database pool.
func New(cfg configuration.Config, pool *pgxpool.Pool) (*Services, error) {
	userStore, err := store.NewUserStore(pool)
	if err != nil {
		return nil, fmt.Errorf("creating user store: %w", err)
	}
	artistTagStore, err := store.NewArtistTagStore(pool)
	if err != nil {
		return nil, fmt.Errorf("creating artist tag store: %w", err)
	}
	venueStore, err := store.NewVenueStore(pool)
	if err != nil {
		return nil, fmt.Errorf("creating venue store: %w", err)
	}

	spotifyClient := spotify.NewClient(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.SpotifyRedirectURI)
	lastfmClient := lastfm.NewClient(cfg.LastFMAPIKey)
	tmClient := ticketmaster.NewClient(cfg.TicketmasterAPIKey)

	tagEnricher, err := service.NewTagEnricher(lastfmClient, artistTagStore)
	if err != nil {
		return nil, fmt.Errorf("creating tag enricher: %w", err)
	}

	authSvc, err := service.NewAuthService(spotifyClient, userStore, cfg.JWTSecret, cfg.TurnstileSecretKey)
	if err != nil {
		return nil, fmt.Errorf("creating auth service: %w", err)
	}

	vibeSvc, err := service.NewVibeService(spotifyClient, userStore, userStore, tagEnricher)
	if err != nil {
		return nil, fmt.Errorf("creating vibe service: %w", err)
	}

	venueSvc, err := service.NewVenueService(tmClient, venueStore, tagEnricher)
	if err != nil {
		return nil, fmt.Errorf("creating venue service: %w", err)
	}

	exploreSvc, err := service.NewExploreService(artistTagStore)
	if err != nil {
		return nil, fmt.Errorf("creating explore service: %w", err)
	}

	return &Services{
		VenueSvc:       venueSvc,
		VibeSvc:        vibeSvc,
		TagEnricher:    tagEnricher,
		AuthSvc:        authSvc,
		ExploreSvc:     exploreSvc,
		UserStore:      userStore,
		ArtistTagStore: artistTagStore,
		VenueStore:     venueStore,
		SpotifyClient:  spotifyClient,
	}, nil
}
