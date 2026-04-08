package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
)

// UserUpserter persists user data on login.
type UserUpserter interface {
	UpsertUser(ctx context.Context, id, displayName, accessToken, refreshToken string, tokenExpiry int) error
}

// SpotifyOAuth handles Spotify OAuth token exchange and profile fetching.
type SpotifyOAuth interface {
	ExchangeCode(ctx context.Context, code string) (*spotify.TokenResponse, error)
	FetchProfile(ctx context.Context, accessToken string) (*spotify.Profile, error)
}

// LoginResult contains the session data produced by a successful login.
type LoginResult struct {
	JWT         string
	UserID      string
	DisplayName string
}

// TurnstileVerifyURL is the Cloudflare Turnstile verification endpoint.
// Exported so tests can override it with a mock server.
var TurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrTurnstileRejected indicates Cloudflare explicitly rejected the token.
var ErrTurnstileRejected = errors.New("turnstile token rejected")

// AuthService handles authentication business logic: Spotify OAuth,
// anonymous login via Turnstile, and user persistence.
type AuthService struct {
	spotify            SpotifyOAuth
	users              UserUpserter
	jwtSecret          string
	turnstileSecretKey string
}

// NewAuthService creates an AuthService.
func NewAuthService(sp SpotifyOAuth, users UserUpserter, jwtSecret, turnstileSecretKey string) (*AuthService, error) {
	if sp == nil {
		return nil, errors.New("auth service: nil spotify client")
	}
	if users == nil {
		return nil, errors.New("auth service: nil user store")
	}
	return &AuthService{
		spotify:            sp,
		users:              users,
		jwtSecret:          jwtSecret,
		turnstileSecretKey: turnstileSecretKey,
	}, nil
}

// LoginWithSpotify exchanges an OAuth code for tokens, fetches the Spotify
// profile, persists the user, and returns a signed JWT.
func (s *AuthService) LoginWithSpotify(ctx context.Context, code string) (*LoginResult, error) {
	tokenResp, err := s.spotify.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	profile, err := s.spotify.FetchProfile(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("fetching profile: %w", err)
	}

	tokenExpiry := int(time.Now().Unix()) + tokenResp.ExpiresIn
	if err := s.users.UpsertUser(ctx, profile.ID, profile.DisplayName, tokenResp.AccessToken, tokenResp.RefreshToken, tokenExpiry); err != nil {
		return nil, fmt.Errorf("persisting user: %w", err)
	}

	jwt, err := auth.CreateToken(s.jwtSecret, profile.ID, profile.DisplayName)
	if err != nil {
		return nil, fmt.Errorf("creating token: %w", err)
	}

	return &LoginResult{
		JWT:         jwt,
		UserID:      profile.ID,
		DisplayName: profile.DisplayName,
	}, nil
}

// LoginAnonymous verifies a Cloudflare Turnstile token, generates an
// anonymous identity, and returns a signed JWT.
func (s *AuthService) LoginAnonymous(ctx context.Context, turnstileToken string) (*LoginResult, error) {
	if err := s.verifyTurnstile(ctx, turnstileToken); err != nil {
		return nil, err
	}

	anonID, err := generateAnonymousID()
	if err != nil {
		return nil, fmt.Errorf("generating anonymous id: %w", err)
	}

	jwt, err := auth.CreateToken(s.jwtSecret, anonID, "Explorer")
	if err != nil {
		return nil, fmt.Errorf("creating anonymous token: %w", err)
	}

	return &LoginResult{
		JWT:         jwt,
		UserID:      anonID,
		DisplayName: "Explorer",
	}, nil
}

// verifyTurnstile validates a Turnstile token with the Cloudflare API.
func (s *AuthService) verifyTurnstile(ctx context.Context, token string) error {
	if s.turnstileSecretKey == "" {
		return fmt.Errorf("turnstile secret key not configured")
	}

	form := url.Values{
		"secret":   {s.turnstileSecretKey},
		"response": {token},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TurnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("creating turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling turnstile API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding turnstile response: %w", err)
	}

	if !result.Success {
		return ErrTurnstileRejected
	}
	return nil
}

// generateAnonymousID creates a random anonymous user identifier.
func generateAnonymousID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "anon-" + hex.EncodeToString(b), nil
}
