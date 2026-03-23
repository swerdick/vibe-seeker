package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
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

type AuthHandler struct {
	Spotify      *spotify.Client
	Users        UserUpserter
	JWTSecret    string
	FrontendURL  string
	SecureCookie bool
}

func NewAuthHandler(spotify *spotify.Client, users UserUpserter, jwtSecret, frontendURL string, secureCookie bool) (*AuthHandler, error) {
	if spotify == nil {
		return nil, errors.New("auth: nil spotify client")
	}
	if users == nil {
		return nil, errors.New("auth: nil user store")
	}
	return &AuthHandler{
		Spotify:      spotify,
		Users:        users,
		JWTSecret:    jwtSecret,
		FrontendURL:  frontendURL,
		SecureCookie: secureCookie,
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
