package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
)

func newTestHandler() *AuthHandler {
	spotify := auth.NewSpotifyClient("client-id", "client-secret", "http://localhost:8080/api/auth/callback")
	return NewAuthHandler(spotify, "jwt-secret", "http://localhost:5173")
}

func TestLogin_RedirectsToSpotify(t *testing.T) {
	h := newTestHandler()

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
	h := newTestHandler()

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
	h := newTestHandler()

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
	h := newTestHandler()

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
	h := newTestHandler()

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
	h := newTestHandler()

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
	h := newTestHandler()

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
