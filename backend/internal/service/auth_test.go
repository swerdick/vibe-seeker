package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/spotify"
)

type mockSpotifyOAuth struct {
	tokenResp  *spotify.TokenResponse
	profile    *spotify.Profile
	exchangeErr error
	profileErr  error
}

func (m *mockSpotifyOAuth) ExchangeCode(_ context.Context, _ string) (*spotify.TokenResponse, error) {
	return m.tokenResp, m.exchangeErr
}

func (m *mockSpotifyOAuth) FetchProfile(_ context.Context, _ string) (*spotify.Profile, error) {
	return m.profile, m.profileErr
}

type mockUserUpserter struct {
	called bool
	err    error
}

func (m *mockUserUpserter) UpsertUser(_ context.Context, _, _, _, _ string, _ int) error {
	m.called = true
	return m.err
}

func TestLoginWithSpotify_Success(t *testing.T) {
	users := &mockUserUpserter{}
	svc, err := NewAuthService(
		&mockSpotifyOAuth{
			tokenResp: &spotify.TokenResponse{
				AccessToken: "at", RefreshToken: "rt", ExpiresIn: 3600,
			},
			profile: &spotify.Profile{ID: "spotify123", DisplayName: "Test User"},
		},
		users, "jwt-secret", "",
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	result, err := svc.LoginWithSpotify(context.Background(), "test-code")
	if err != nil {
		t.Fatalf("LoginWithSpotify: %v", err)
	}

	if result.UserID != "spotify123" {
		t.Errorf("UserID = %q, want %q", result.UserID, "spotify123")
	}
	if result.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Test User")
	}
	if result.JWT == "" {
		t.Error("expected non-empty JWT")
	}
	if !users.called {
		t.Error("expected UpsertUser to be called")
	}
}

func TestLoginWithSpotify_ExchangeError(t *testing.T) {
	svc, _ := NewAuthService(
		&mockSpotifyOAuth{exchangeErr: errors.New("bad code")},
		&mockUserUpserter{}, "jwt-secret", "",
	)

	_, err := svc.LoginWithSpotify(context.Background(), "bad-code")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exchanging code") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoginWithSpotify_ProfileError(t *testing.T) {
	svc, _ := NewAuthService(
		&mockSpotifyOAuth{
			tokenResp:  &spotify.TokenResponse{AccessToken: "at"},
			profileErr: errors.New("profile error"),
		},
		&mockUserUpserter{}, "jwt-secret", "",
	)

	_, err := svc.LoginWithSpotify(context.Background(), "code")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fetching profile") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoginWithSpotify_UpsertError(t *testing.T) {
	svc, _ := NewAuthService(
		&mockSpotifyOAuth{
			tokenResp: &spotify.TokenResponse{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 3600},
			profile:   &spotify.Profile{ID: "id1", DisplayName: "Name"},
		},
		&mockUserUpserter{err: errors.New("db error")}, "jwt-secret", "",
	)

	_, err := svc.LoginWithSpotify(context.Background(), "code")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "persisting user") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoginAnonymous_Success(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer mock.Close()

	original := TurnstileVerifyURL
	TurnstileVerifyURL = mock.URL
	defer func() { TurnstileVerifyURL = original }()

	svc, _ := NewAuthService(
		&mockSpotifyOAuth{},
		&mockUserUpserter{}, "jwt-secret", "test-secret",
	)

	result, err := svc.LoginAnonymous(context.Background(), "token")
	if err != nil {
		t.Fatalf("LoginAnonymous: %v", err)
	}

	if !strings.HasPrefix(result.UserID, "anon-") {
		t.Errorf("expected anonymous UserID prefix, got %q", result.UserID)
	}
	if result.DisplayName != "Explorer" {
		t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Explorer")
	}
	if result.JWT == "" {
		t.Error("expected non-empty JWT")
	}
}

func TestLoginAnonymous_TurnstileRejected(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": false})
	}))
	defer mock.Close()

	original := TurnstileVerifyURL
	TurnstileVerifyURL = mock.URL
	defer func() { TurnstileVerifyURL = original }()

	svc, _ := NewAuthService(
		&mockSpotifyOAuth{},
		&mockUserUpserter{}, "jwt-secret", "test-secret",
	)

	_, err := svc.LoginAnonymous(context.Background(), "bad-token")
	if !errors.Is(err, ErrTurnstileRejected) {
		t.Errorf("expected ErrTurnstileRejected, got %v", err)
	}
}

func TestLoginAnonymous_NoSecretKey(t *testing.T) {
	svc, _ := NewAuthService(
		&mockSpotifyOAuth{},
		&mockUserUpserter{}, "jwt-secret", "",
	)

	_, err := svc.LoginAnonymous(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error for missing secret key")
	}
}

func TestNewAuthService_NilSpotify(t *testing.T) {
	_, err := NewAuthService(nil, &mockUserUpserter{}, "secret", "")
	if err == nil {
		t.Error("expected error for nil spotify")
	}
}

func TestNewAuthService_NilUsers(t *testing.T) {
	_, err := NewAuthService(&mockSpotifyOAuth{}, nil, "secret", "")
	if err == nil {
		t.Error("expected error for nil users")
	}
}
