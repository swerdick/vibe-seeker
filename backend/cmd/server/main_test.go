package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pseudo/vibe-seeker/backend/internal/utility"
)

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		got := setLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("setLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNewServer_HealthEndpoint(t *testing.T) {
	cfg := utility.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:5173",
	}

	server := newServer(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

func TestNewServer_CORSHeaders(t *testing.T) {
	cfg := utility.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:3000",
	}

	server := newServer(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	got := res.Header.Get("Access-Control-Allow-Origin")
	if got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
	}
}

func TestNewServer_PreflightRequest(t *testing.T) {
	cfg := utility.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:5173",
	}

	server := newServer(cfg)

	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", res.StatusCode)
	}
}
