package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRelayTraces_Disabled(t *testing.T) {
	h := NewOTLPRelayHandler("", "", "", false)
	req := httptest.NewRequest(http.MethodPost, "/api/otlp/v1/traces", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	h.RelayTraces(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestRelayTraces_ForwardsToUpstream(t *testing.T) {
	var gotBody string
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	h := NewOTLPRelayHandler(upstream.URL, "", "Authorization=Basic dGVzdDp0ZXN0", true)

	body := `{"resourceSpans":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/otlp/v1/traces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.RelayTraces(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotBody != body {
		t.Fatalf("upstream got body %q, want %q", gotBody, body)
	}
	if gotAuth != "Basic dGVzdDp0ZXN0" {
		t.Fatalf("upstream got auth %q, want %q", gotAuth, "Basic dGVzdDp0ZXN0")
	}
}

func TestRelayTraces_FallsBackToGRPCEndpoint(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	// relayEndpoint empty → falls back to grpcEndpoint
	h := NewOTLPRelayHandler("", upstream.URL, "", true)

	req := httptest.NewRequest(http.MethodPost, "/api/otlp/v1/traces", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.RelayTraces(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotPath != "/v1/traces" {
		t.Fatalf("expected upstream path /v1/traces, got %q", gotPath)
	}
}

func TestRelayTraces_DisabledWhenNoEndpoint(t *testing.T) {
	h := NewOTLPRelayHandler("", "", "", true)
	req := httptest.NewRequest(http.MethodPost, "/api/otlp/v1/traces", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	h.RelayTraces(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 (auto-disabled), got %d", w.Code)
	}
}

func TestRelayTraces_OversizedBody(t *testing.T) {
	h := NewOTLPRelayHandler("http://localhost", "", "", true)
	bigBody := strings.Repeat("x", maxOTLPBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/api/otlp/v1/traces", strings.NewReader(bigBody))
	w := httptest.NewRecorder()

	h.RelayTraces(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

func TestParseOTLPHeaders(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]string
	}{
		{"", map[string]string{}},
		{"Authorization=Basic abc123", map[string]string{"Authorization": "Basic abc123"}},
		{"X-Foo=bar,X-Baz=qux", map[string]string{"X-Foo": "bar", "X-Baz": "qux"}},
	}
	for _, tt := range tests {
		got := parseOTLPHeaders(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseOTLPHeaders(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("parseOTLPHeaders(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
			}
		}
	}
}
