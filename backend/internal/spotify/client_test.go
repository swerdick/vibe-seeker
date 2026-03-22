package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAuthorizeURL_ContainsRequiredParams(t *testing.T) {
	c := NewClient("my-client-id", "my-secret", "http://localhost:8080/callback")

	raw := c.AuthorizeURL("test-state")

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	if u.Host != "accounts.spotify.com" {
		t.Errorf("host = %q, want accounts.spotify.com", u.Host)
	}

	tests := []struct {
		param string
		want  string
	}{
		{"client_id", "my-client-id"},
		{"response_type", "code"},
		{"redirect_uri", "http://localhost:8080/callback"},
		{"scope", Scopes},
		{"state", "test-state"},
	}

	query := u.Query()
	for _, tt := range tests {
		got := query.Get(tt.param)
		if got != tt.want {
			t.Errorf("param %s = %q, want %q", tt.param, got, tt.want)
		}
	}
}

func TestExchangeCode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		user, pass, ok := r.BasicAuth()
		if !ok || user != "client-id" || pass != "client-secret" {
			t.Errorf("bad basic auth: user=%q pass=%q ok=%v", user, pass, ok)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "test-code" {
			t.Errorf("code = %q, want test-code", r.FormValue("code"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "mock-access-token",
			"refresh_token": "mock-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TokenURL = server.URL

	tokenResp, err := c.ExchangeCode(context.Background(),"test-code")
	if err != nil {
		t.Fatalf("ExchangeCode failed: %v", err)
	}

	if tokenResp.AccessToken != "mock-access-token" {
		t.Errorf("AccessToken = %q, want mock-access-token", tokenResp.AccessToken)
	}
	if tokenResp.RefreshToken != "mock-refresh-token" {
		t.Errorf("RefreshToken = %q, want mock-refresh-token", tokenResp.RefreshToken)
	}
	if tokenResp.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", tokenResp.ExpiresIn)
	}
}

func TestExchangeCode_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TokenURL = server.URL

	_, err := c.ExchangeCode(context.Background(),"bad-code")
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}

func TestFetchProfile_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer mock-token" {
			t.Errorf("Authorization = %q, want Bearer mock-token", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":           "spotify-user-1",
			"display_name": "Test User",
		})
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.MeURL = server.URL

	profile, err := c.FetchProfile(context.Background(),"mock-token")
	if err != nil {
		t.Fatalf("FetchProfile failed: %v", err)
	}

	if profile.ID != "spotify-user-1" {
		t.Errorf("ID = %q, want spotify-user-1", profile.ID)
	}

	if profile.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want Test User", profile.DisplayName)
	}
}

func TestFetchProfile_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.MeURL = server.URL

	_, err := c.FetchProfile(context.Background(),"bad-token")
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}
