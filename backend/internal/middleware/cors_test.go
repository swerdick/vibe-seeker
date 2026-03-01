package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_SetsHeaders(t *testing.T) {
	origin := "http://localhost:5173"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(origin)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	tests := []struct {
		header string
		want   string
	}{
		{"Access-Control-Allow-Origin", origin},
		{"Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS"},
		{"Access-Control-Allow-Headers", "Content-Type, Authorization"},
		{"Access-Control-Max-Age", "86400"},
	}

	for _, tt := range tests {
		got := res.Header.Get(tt.header)
		if got != tt.want {
			t.Errorf("%s = %q, want %q", tt.header, got, tt.want)
		}
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
}

func TestCORS_PreflightReturns204(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := CORS("http://localhost:5173")(inner)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", res.StatusCode)
	}

	if called {
		t.Error("inner handler should not be called for preflight requests")
	}
}

func TestCORS_CustomOrigin(t *testing.T) {
	origin := "https://vibe-seeker.example.com"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS(origin)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Result().Header.Get("Access-Control-Allow-Origin")
	if got != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
}
