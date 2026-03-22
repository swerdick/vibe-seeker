package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefreshToken_Success(t *testing.T) {
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
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old-refresh-token" {
			t.Errorf("refresh_token = %q, want old-refresh-token", r.FormValue("refresh_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TokenURL = server.URL

	resp, err := c.RefreshToken(context.Background(),"old-refresh-token")
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if resp.AccessToken != "new-access-token" {
		t.Errorf("AccessToken = %q, want new-access-token", resp.AccessToken)
	}
	if resp.RefreshToken != "new-refresh-token" {
		t.Errorf("RefreshToken = %q, want new-refresh-token", resp.RefreshToken)
	}
}

func TestRefreshToken_PreservesOldRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-access-token",
			"expires_in":   3600,
			// No refresh_token in response — Spotify doesn't always rotate it.
		})
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TokenURL = server.URL

	resp, err := c.RefreshToken(context.Background(),"original-refresh-token")
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if resp.RefreshToken != "original-refresh-token" {
		t.Errorf("RefreshToken = %q, want original-refresh-token (preserved)", resp.RefreshToken)
	}
}

func TestRefreshToken_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	c := NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TokenURL = server.URL

	_, err := c.RefreshToken(context.Background(),"bad-token")
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}
