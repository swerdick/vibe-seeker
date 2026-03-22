package webserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pseudo/vibe-seeker/backend/internal/configuration"
)

func TestParseLogLevel(t *testing.T) {
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
		got := ParseLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNew_NilPool(t *testing.T) {
	cfg := configuration.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:5173",
	}

	_, err := New(cfg, nil)
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

func TestNew_HealthEndpoint(t *testing.T) {
	cfg := configuration.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:5173",
	}

	server, err := New(cfg, &pgxpool.Pool{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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

func TestNew_CORSHeaders(t *testing.T) {
	cfg := configuration.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:3000",
	}

	server, err := New(cfg, &pgxpool.Pool{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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

func TestNew_PreflightRequest(t *testing.T) {
	cfg := configuration.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:5173",
	}

	server, err := New(cfg, &pgxpool.Pool{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", res.StatusCode)
	}
}

func TestNew_VibeRoutesRequireAuth(t *testing.T) {
	cfg := configuration.Config{
		AppName:    "test-app",
		Port:       0,
		CORSOrigin: "http://localhost:5173",
	}

	server, err := New(cfg, &pgxpool.Pool{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/vibe/sync"},
		{http.MethodGet, "/api/vibe"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", tt.method, tt.path, rec.Code)
		}
	}
}
