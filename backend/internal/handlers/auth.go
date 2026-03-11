package handlers

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
)

type AuthHandler struct {
	Spotify     *auth.SpotifyClient
	JWTSecret   string
	FrontendURL string
}

func NewAuthHandler(spotify *auth.SpotifyClient, jwtSecret, frontendURL string) *AuthHandler {
	return &AuthHandler{
		Spotify:     spotify,
		JWTSecret:   jwtSecret,
		FrontendURL: frontendURL,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := auth.GenerateState()
	if err != nil {
		slog.Error("failed to generate oauth state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
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
		slog.Error("spotify auth error", "error", errParam)
		http.Redirect(w, r, h.FrontendURL+"/?error="+url.QueryEscape(errParam), http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	accessToken, err := h.Spotify.ExchangeCode(code)
	if err != nil {
		slog.Error("failed to exchange code", "error", err)
		http.Error(w, "failed to exchange code", http.StatusInternalServerError)
		return
	}

	profile, err := h.Spotify.FetchProfile(accessToken)
	if err != nil {
		slog.Error("failed to fetch profile", "error", err)
		http.Error(w, "failed to fetch profile", http.StatusInternalServerError)
		return
	}

	token, err := auth.CreateToken(h.JWTSecret, profile.ID, profile.DisplayName)
	if err != nil {
		slog.Error("failed to create token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.FrontendURL+"/callback?token="+url.QueryEscape(token), http.StatusFound)
}
