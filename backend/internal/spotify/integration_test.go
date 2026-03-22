package spotify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/auth"
	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
)

// TestFullOAuthFlow tests the complete code exchange → profile fetch → JWT creation chain.
func TestFullOAuthFlow(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "mock-access-token",
			"refresh_token": "mock-refresh-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	meServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mock-access-token" {
			t.Errorf("me endpoint: expected Bearer mock-access-token, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":           "spotify-user-1",
			"display_name": "Test User",
		})
	}))
	defer meServer.Close()

	c := spotify.NewClient("client-id", "client-secret", "http://localhost:8080/callback")
	c.TokenURL = tokenServer.URL
	c.MeURL = meServer.URL

	tokenResp, err := c.ExchangeCode(context.Background(), "auth-code")
	if err != nil {
		t.Fatalf("ExchangeCode failed: %v", err)
	}

	profile, err := c.FetchProfile(context.Background(), tokenResp.AccessToken)
	if err != nil {
		t.Fatalf("FetchProfile failed: %v", err)
	}

	token, err := auth.CreateToken("jwt-secret", profile.ID, profile.DisplayName)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	claims, err := auth.ParseToken("jwt-secret", token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.SpotifyID != "spotify-user-1" {
		t.Errorf("SpotifyID = %q, want spotify-user-1", claims.SpotifyID)
	}

	if claims.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want Test User", claims.DisplayName)
	}
}
