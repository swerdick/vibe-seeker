package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
)

// mockUserStore implements UserUpserter for tests.
type mockUserStore struct {
	called      bool
	lastID      string
	lastDisplay string
	err         error
}

func (m *mockUserStore) UpsertUser(_ context.Context, id, displayName, _, _ string, _ int) error {
	m.called = true
	m.lastID = id
	m.lastDisplay = displayName
	return m.err
}

func newTestHandler(t *testing.T) *AuthHandler {
	t.Helper()
	spotify := auth.NewSpotifyClient("client-id", "client-secret", "http://localhost:8080/api/auth/callback")
	h, err := NewAuthHandler(spotify, &mockUserStore{}, "jwt-secret", "http://localhost:5173", false)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}
	return h
}

func TestLogin_RedirectsToSpotify(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusFound {
		t.Fatalf("expected status 302, got %d", res.StatusCode)
	}

	location, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatalf("failed to parse Location header: %v", err)
	}

	if location.Host != "accounts.spotify.com" {
		t.Errorf("expected redirect to accounts.spotify.com, got %s", location.Host)
	}

	if location.Query().Get("state") == "" {
		t.Error("expected non-empty state parameter")
	}
}

func TestLogin_SetsStateCookie(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	var stateCookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}

	if stateCookie == nil {
		t.Fatal("expected oauth_state cookie to be set")
		return
	}

	if stateCookie.Value == "" {
		t.Error("expected non-empty oauth_state cookie value")
	}

	if !stateCookie.HttpOnly {
		t.Error("expected oauth_state cookie to be HttpOnly")
	}
}

func TestCallback_InvalidState(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=bad-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "good-state"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", res.StatusCode)
	}
}

func TestCallback_MissingStateCookie(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=some-state&code=test-code", nil)
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", res.StatusCode)
	}
}

func TestCallback_SpotifyError(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&error=access_denied", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusFound {
		t.Fatalf("expected status 302, got %d", res.StatusCode)
	}

	location := res.Header.Get("Location")
	if location != "http://localhost:5173/?error=access_denied" {
		t.Errorf("unexpected redirect: %s", location)
	}
}

func TestCallback_MissingCode(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", res.StatusCode)
	}
}

func TestCallback_ClearsStateCookie(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&error=access_denied", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	var cleared bool
	for _, c := range res.Cookies() {
		if c.Name == "oauth_state" && c.MaxAge == -1 {
			cleared = true
			break
		}
	}

	if !cleared {
		t.Error("expected oauth_state cookie to be cleared")
	}
}

func TestMe_ReturnsUserInfo(t *testing.T) {
	h := newTestHandler(t)

	token, err := auth.CreateToken("jwt-secret", "spotify123", "Test User")
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	// Wrap the Me handler with auth middleware.
	handler := middleware.RequireAuth("jwt-secret")(http.HandlerFunc(h.Me))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !contains(body, "spotify123") {
		t.Errorf("response should contain spotify_id, got: %s", body)
	}
	if !contains(body, "Test User") {
		t.Errorf("response should contain display_name, got: %s", body)
	}
}

func TestMe_Unauthorized(t *testing.T) {
	h := newTestHandler(t)

	handler := middleware.RequireAuth("jwt-secret")(http.HandlerFunc(h.Me))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestLogout_ClearsSessionCookie(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", res.StatusCode)
	}

	var cleared bool
	for _, c := range res.Cookies() {
		if c.Name == "session" && c.MaxAge == -1 {
			cleared = true
			break
		}
	}

	if !cleared {
		t.Error("expected session cookie to be cleared")
	}
}

// newTestHandlerWithMockSpotify creates an AuthHandler backed by a mock Spotify API
// that returns a valid token exchange and profile response.
func newTestHandlerWithMockSpotify(t *testing.T) (*AuthHandler, *mockUserStore, *httptest.Server) {
	t.Helper()

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "mock-access-token",
				"refresh_token": "mock-refresh-token",
				"expires_in":    3600,
			})
		case "/v1/me":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "spotify123", "display_name": "Test User"})
		default:
			http.NotFound(w, r)
		}
	}))

	spotify := auth.NewSpotifyClient("client-id", "client-secret", "http://localhost:5173/api/auth/callback")
	spotify.TokenURL = mock.URL + "/api/token"
	spotify.MeURL = mock.URL + "/v1/me"
	spotify.HTTPClient = mock.Client()

	users := &mockUserStore{}
	h, err := NewAuthHandler(spotify, users, "jwt-secret", "http://localhost:5173", false)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}
	return h, users, mock
}

func TestCallback_Success_SetsSessionCookie(t *testing.T) {
	h, _, mock := newTestHandlerWithMockSpotify(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusFound {
		t.Fatalf("expected status 302, got %d", res.StatusCode)
	}

	var sessionCookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}

	if sessionCookie.Value == "" {
		t.Error("expected non-empty session cookie value")
	}

	// Verify the cookie contains a valid JWT.
	claims, err := auth.ParseToken("jwt-secret", sessionCookie.Value)
	if err != nil {
		t.Fatalf("session cookie is not a valid JWT: %v", err)
	}

	if claims.SpotifyID != "spotify123" {
		t.Errorf("SpotifyID = %q, want %q", claims.SpotifyID, "spotify123")
	}

	if claims.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", claims.DisplayName, "Test User")
	}
}

func TestCallback_Success_CookieProperties(t *testing.T) {
	h, _, mock := newTestHandlerWithMockSpotify(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	var sessionCookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}

	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}

	if sessionCookie.Path != "/api" {
		t.Errorf("session cookie Path = %q, want %q", sessionCookie.Path, "/api")
	}

	if sessionCookie.MaxAge != 86400 {
		t.Errorf("session cookie MaxAge = %d, want %d", sessionCookie.MaxAge, 86400)
	}

	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie SameSite = %d, want %d (Lax)", sessionCookie.SameSite, http.SameSiteLaxMode)
	}

	// SecureCookie is false in test handler, so Secure should be false.
	if sessionCookie.Secure {
		t.Error("session cookie should not be Secure in test (local) mode")
	}
}

func TestCallback_Success_UpsertsUser(t *testing.T) {
	h, users, mock := newTestHandlerWithMockSpotify(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if !users.called {
		t.Fatal("expected UpsertUser to be called")
	}
	if users.lastID != "spotify123" {
		t.Errorf("UpsertUser id = %q, want %q", users.lastID, "spotify123")
	}
	if users.lastDisplay != "Test User" {
		t.Errorf("UpsertUser displayName = %q, want %q", users.lastDisplay, "Test User")
	}
}

func TestCallback_Success_RedirectsToFrontend(t *testing.T) {
	h, _, mock := newTestHandlerWithMockSpotify(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	location := res.Header.Get("Location")
	if location != "http://localhost:5173/callback" {
		t.Errorf("expected redirect to frontend callback, got %s", location)
	}
}

func TestCallback_ExchangeCodeFailure(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer mock.Close()

	spotify := auth.NewSpotifyClient("client-id", "client-secret", "http://localhost:5173/api/auth/callback")
	spotify.TokenURL = mock.URL + "/api/token"
	spotify.HTTPClient = mock.Client()
	h, err := NewAuthHandler(spotify, &mockUserStore{}, "jwt-secret", "http://localhost:5173", false)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=valid&code=bad-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "valid"})
	rec := httptest.NewRecorder()

	h.Callback(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestLogout_CookieProperties(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	var sessionCookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("expected session cookie in logout response")
	}

	if !sessionCookie.HttpOnly {
		t.Error("logout cookie must be HttpOnly")
	}

	if sessionCookie.Path != "/api" {
		t.Errorf("logout cookie Path = %q, want %q", sessionCookie.Path, "/api")
	}
}

func TestMe_ResponseFormat(t *testing.T) {
	h := newTestHandler(t)

	token, err := auth.CreateToken("jwt-secret", "spotify123", "Test User")
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	handler := middleware.RequireAuth("jwt-secret")(http.HandlerFunc(h.Me))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["spotify_id"] != "spotify123" {
		t.Errorf("spotify_id = %q, want %q", body["spotify_id"], "spotify123")
	}

	if body["display_name"] != "Test User" {
		t.Errorf("display_name = %q, want %q", body["display_name"], "Test User")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
