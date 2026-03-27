package handlers

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
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
)

// UserUpserter persists user data on login.
type UserUpserter interface {
	UpsertUser(ctx context.Context, id, displayName, accessToken, refreshToken string, tokenExpiry int) error
}

// TurnstileVerifyURL is the Cloudflare Turnstile verification endpoint.
// Exported so tests can override it with a mock server.
var TurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type AuthHandler struct {
	Spotify            *spotify.Client
	Users              UserUpserter
	JWTSecret          string
	FrontendURL        string
	TurnstileSecretKey string
	SecureCookie       bool
}

func NewAuthHandler(spotify *spotify.Client, users UserUpserter, jwtSecret, frontendURL, turnstileSecretKey string, secureCookie bool) (*AuthHandler, error) {
	if spotify == nil {
		return nil, errors.New("auth: nil spotify client")
	}
	if users == nil {
		return nil, errors.New("auth: nil user store")
	}
	return &AuthHandler{
		Spotify:            spotify,
		Users:              users,
		JWTSecret:          jwtSecret,
		FrontendURL:        frontendURL,
		TurnstileSecretKey: turnstileSecretKey,
		SecureCookie:       secureCookie,
	}, nil
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := auth.GenerateState()
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to generate oauth state", "error", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   configuration.OAuthStateCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.Spotify.AuthorizeURL(state), http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		observability.Logger(r.Context()).Error("spotify auth error", "error", errParam)
		http.Redirect(w, r, h.FrontendURL+"/?error="+url.QueryEscape(errParam), http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	tokenResp, err := h.Spotify.ExchangeCode(r.Context(), code)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "failed to exchange code",
			"failed to exchange code", "error", err)
		return
	}

	profile, err := h.Spotify.FetchProfile(r.Context(), tokenResp.AccessToken)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "failed to fetch profile",
			"failed to fetch profile", "error", err)
		return
	}

	tokenExpiry := int(time.Now().Unix()) + tokenResp.ExpiresIn
	if err := h.Users.UpsertUser(r.Context(), profile.ID, profile.DisplayName, tokenResp.AccessToken, tokenResp.RefreshToken, tokenExpiry); err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to persist user", "error", err)
		return
	}

	jwt, err := auth.CreateToken(h.JWTSecret, profile.ID, profile.DisplayName)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to create token", "error", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    jwt,
		Path:     "/api",
		MaxAge:   configuration.SessionCookieMaxAge,
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.FrontendURL+"/callback", http.StatusFound)
}

// Me returns the authenticated user's identity from the JWT claims.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"spotify_id":   claims.SpotifyID,
		"display_name": claims.DisplayName,
	})
}

// AnonymousLogin validates a Cloudflare Turnstile token and issues a
// session cookie with an anonymous JWT. No user row is created.
func (h *AuthHandler) AnonymousLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TurnstileToken string `json:"turnstile_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TurnstileToken == "" {
		http.Error(w, "missing turnstile_token", http.StatusBadRequest)
		return
	}

	if err := h.verifyTurnstile(r.Context(), body.TurnstileToken); err != nil {
		httpError(w, r, http.StatusForbidden, "captcha verification failed",
			"turnstile verification failed", "error", err)
		return
	}

	anonID, err := generateAnonymousID()
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to generate anonymous id", "error", err)
		return
	}

	jwt, err := auth.CreateToken(h.JWTSecret, anonID, "Explorer")
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "internal error",
			"failed to create anonymous token", "error", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    jwt,
		Path:     "/api",
		MaxAge:   configuration.SessionCookieMaxAge,
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"spotify_id":   anonID,
		"display_name": "Explorer",
	})
}

// verifyTurnstile validates a Turnstile token with the Cloudflare API.
func (h *AuthHandler) verifyTurnstile(ctx context.Context, token string) error {
	form := url.Values{
		"secret":   {h.TurnstileSecretKey},
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
		return fmt.Errorf("turnstile token rejected")
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

// Logout clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/api",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusNoContent)
}
