package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/observability"
	"github.com/pseudo/vibe-seeker/backend/internal/service"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
)

// Authenticator handles the business logic of user authentication.
type Authenticator interface {
	LoginWithSpotify(ctx context.Context, code string) (*service.LoginResult, error)
	LoginAnonymous(ctx context.Context, turnstileToken string) (*service.LoginResult, error)
}

// AuthHandler handles HTTP authentication endpoints.
type AuthHandler struct {
	auth         Authenticator
	spotify      *spotify.Client // only for AuthorizeURL in Login
	frontendURL  string
	secureCookie bool
}

func NewAuthHandler(sp *spotify.Client, authSvc Authenticator, frontendURL string, secureCookie bool) (*AuthHandler, error) {
	if sp == nil {
		return nil, errors.New("auth: nil spotify client")
	}
	if authSvc == nil {
		return nil, errors.New("auth: nil auth service")
	}
	return &AuthHandler{
		auth:         authSvc,
		spotify:      sp,
		frontendURL:  frontendURL,
		secureCookie: secureCookie,
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

	http.Redirect(w, r, h.spotify.AuthorizeURL(state), http.StatusFound)
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
		http.Redirect(w, r, h.frontendURL+"/?error="+url.QueryEscape(errParam), http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	result, err := h.auth.LoginWithSpotify(r.Context(), code)
	if err != nil {
		httpError(w, r, http.StatusInternalServerError, "failed to complete login",
			"spotify login failed", "error", err)
		return
	}

	h.setSessionCookie(w, result.JWT)
	http.Redirect(w, r, h.frontendURL+"/callback", http.StatusFound)
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
// session cookie with an anonymous JWT.
func (h *AuthHandler) AnonymousLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TurnstileToken string `json:"turnstile_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TurnstileToken == "" {
		http.Error(w, "missing turnstile_token", http.StatusBadRequest)
		return
	}

	result, err := h.auth.LoginAnonymous(r.Context(), body.TurnstileToken)
	if err != nil {
		if errors.Is(err, service.ErrTurnstileRejected) {
			httpError(w, r, http.StatusForbidden, "captcha verification failed",
				"turnstile token rejected", "error", err)
		} else {
			httpError(w, r, http.StatusInternalServerError, "captcha verification failed",
				"turnstile verification error", "error", err)
		}
		return
	}

	h.setSessionCookie(w, result.JWT)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"spotify_id":   result.UserID,
		"display_name": result.DisplayName,
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
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, jwt string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    jwt,
		Path:     "/api",
		MaxAge:   configuration.SessionCookieMaxAge,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}
