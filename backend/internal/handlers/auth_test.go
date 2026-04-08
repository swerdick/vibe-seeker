package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/middleware"
	"github.com/pseudo/vibe-seeker/backend/internal/service"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
)

// mockAuthService implements Authenticator for tests.
type mockAuthService struct {
	loginResult *service.LoginResult
	loginErr    error
	anonResult  *service.LoginResult
	anonErr     error
}

func (m *mockAuthService) LoginWithSpotify(_ context.Context, _ string) (*service.LoginResult, error) {
	return m.loginResult, m.loginErr
}

func (m *mockAuthService) LoginAnonymous(_ context.Context, _ string) (*service.LoginResult, error) {
	return m.anonResult, m.anonErr
}

func newTestHandler(t *testing.T) *AuthHandler {
	t.Helper()
	sp := spotify.NewClient("client-id", "client-secret", "http://localhost:8080/api/auth/callback")
	jwt, _ := auth.CreateToken("jwt-secret", "spotify123", "Test User")
	h, err := NewAuthHandler(sp, &mockAuthService{
		loginResult: &service.LoginResult{JWT: jwt, UserID: "spotify123", DisplayName: "Test User"},
		anonResult:  &service.LoginResult{JWT: jwt, UserID: "anon-abc123", DisplayName: "Explorer"},
	}, "http://localhost:5173", false)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}
	return h
}

func newTestHandlerWithMockSpotify(t *testing.T) (*AuthHandler, *mockAuthService) {
	t.Helper()

	sp := spotify.NewClient("client-id", "client-secret", "http://localhost:5173/api/auth/callback")

	jwt, _ := auth.CreateToken("jwt-secret", "spotify123", "Test User")
	authSvc := &mockAuthService{
		loginResult: &service.LoginResult{JWT: jwt, UserID: "spotify123", DisplayName: "Test User"},
	}

	h, err := NewAuthHandler(sp, authSvc, "http://localhost:5173", false)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}
	return h, authSvc
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

	location := res.Header.Get("Location")
	if !strings.Contains(location, "accounts.spotify.com") {
		t.Errorf("expected redirect to accounts.spotify.com, got %s", location)
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

	handler := middleware.RequireAuth("jwt-secret")(http.HandlerFunc(h.Me))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "spotify123") {
		t.Errorf("response should contain spotify_id, got: %s", body)
	}
	if !strings.Contains(body, "Test User") {
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

func TestCallback_Success_SetsSessionCookie(t *testing.T) {
	h, _ := newTestHandlerWithMockSpotify(t)

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
	h, _ := newTestHandlerWithMockSpotify(t)

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

func TestCallback_Success_RedirectsToFrontend(t *testing.T) {
	h, _ := newTestHandlerWithMockSpotify(t)

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

func TestCallback_ServiceFailure(t *testing.T) {
	sp := spotify.NewClient("client-id", "client-secret", "http://localhost:5173/api/auth/callback")
	h, err := NewAuthHandler(sp, &mockAuthService{
		loginErr: context.DeadlineExceeded,
	}, "http://localhost:5173", false)
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

// --- Anonymous login tests ---

func newAnonymousTestHandler(t *testing.T, anonResult *service.LoginResult, anonErr error) *AuthHandler {
	t.Helper()
	sp := spotify.NewClient("client-id", "client-secret", "http://localhost:8080/api/auth/callback")
	h, err := NewAuthHandler(sp, &mockAuthService{
		anonResult: anonResult,
		anonErr:    anonErr,
	}, "http://localhost:5173", false)
	if err != nil {
		t.Fatalf("NewAuthHandler: %v", err)
	}
	return h
}

func TestAnonymousLogin_Success(t *testing.T) {
	jwt, _ := auth.CreateToken("jwt-secret", "anon-abc123", "Explorer")
	h := newAnonymousTestHandler(t, &service.LoginResult{
		JWT: jwt, UserID: "anon-abc123", DisplayName: "Explorer",
	}, nil)

	body := strings.NewReader(`{"turnstile_token":"test-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/anonymous", body)
	rec := httptest.NewRecorder()

	h.AnonymousLogin(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	// Verify session cookie is set.
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

	// Verify JWT contains anonymous ID.
	claims, err := auth.ParseToken("jwt-secret", sessionCookie.Value)
	if err != nil {
		t.Fatalf("session cookie is not a valid JWT: %v", err)
	}
	if claims.SpotifyID != "anon-abc123" {
		t.Errorf("expected anonymous SpotifyID, got %q", claims.SpotifyID)
	}
	if claims.DisplayName != "Explorer" {
		t.Errorf("DisplayName = %q, want %q", claims.DisplayName, "Explorer")
	}

	// Verify response body.
	var respBody map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&respBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if respBody["display_name"] != "Explorer" {
		t.Errorf("response display_name = %q, want %q", respBody["display_name"], "Explorer")
	}
}

func TestAnonymousLogin_TurnstileRejected(t *testing.T) {
	h := newAnonymousTestHandler(t, nil, service.ErrTurnstileRejected)

	body := strings.NewReader(`{"turnstile_token":"bad-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/anonymous", body)
	rec := httptest.NewRecorder()

	h.AnonymousLogin(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}
}

func TestAnonymousLogin_MissingToken(t *testing.T) {
	h := newTestHandler(t)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/anonymous", body)
	rec := httptest.NewRecorder()

	h.AnonymousLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAnonymousLogin_CookieProperties(t *testing.T) {
	jwt, _ := auth.CreateToken("jwt-secret", "anon-abc123", "Explorer")
	h := newAnonymousTestHandler(t, &service.LoginResult{
		JWT: jwt, UserID: "anon-abc123", DisplayName: "Explorer",
	}, nil)

	body := strings.NewReader(`{"turnstile_token":"test-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/anonymous", body)
	rec := httptest.NewRecorder()

	h.AnonymousLogin(rec, req)

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
		t.Fatal("expected session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if sessionCookie.Path != "/api" {
		t.Errorf("session cookie Path = %q, want %q", sessionCookie.Path, "/api")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie SameSite = %d, want %d (Lax)", sessionCookie.SameSite, http.SameSiteLaxMode)
	}
}

func TestAnonymousLogin_JWTPassesRequireAuth(t *testing.T) {
	jwt, _ := auth.CreateToken("jwt-secret", "anon-abc123", "Explorer")
	h := newAnonymousTestHandler(t, &service.LoginResult{
		JWT: jwt, UserID: "anon-abc123", DisplayName: "Explorer",
	}, nil)

	// Get an anonymous session cookie.
	body := strings.NewReader(`{"turnstile_token":"test-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/anonymous", body)
	rec := httptest.NewRecorder()
	h.AnonymousLogin(rec, req)

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie")
	}

	// Use the anonymous cookie to call a protected endpoint (Me).
	meHandler := middleware.RequireAuth("jwt-secret")(http.HandlerFunc(h.Me))
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()

	meHandler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected anonymous JWT to pass RequireAuth, got status %d", meRec.Code)
	}

	var meBody map[string]string
	if err := json.NewDecoder(meRec.Body).Decode(&meBody); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}
	if meBody["spotify_id"] != "anon-abc123" {
		t.Errorf("expected anonymous spotify_id, got %q", meBody["spotify_id"])
	}
}
