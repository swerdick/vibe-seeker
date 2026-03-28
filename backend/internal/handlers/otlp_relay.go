package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const maxOTLPBodySize = 512 * 1024 // 512KB

// OTLPRelayHandler forwards browser OTLP telemetry to the configured collector.
// This keeps API keys server-side and avoids CORS issues.
type OTLPRelayHandler struct {
	endpoint string
	headers  map[string]string
	enabled  bool
	client   *http.Client
}

// NewOTLPRelayHandler creates a relay that forwards OTLP HTTP/JSON to the given
// endpoint. The relayEndpoint is an HTTP endpoint (e.g. http://localhost:4318);
// if empty, grpcEndpoint is used as a fallback (works for gateways like Grafana
// Cloud that accept both protocols on the same origin).
// The headers string uses the standard OTel format: "key=value,key2=value2".
func NewOTLPRelayHandler(relayEndpoint, grpcEndpoint, headers string, enabled bool) *OTLPRelayHandler {
	endpoint := relayEndpoint
	if endpoint == "" {
		endpoint = grpcEndpoint
	}
	if enabled && endpoint == "" {
		slog.Warn("otlp relay: enabled but no endpoint configured, disabling")
		enabled = false
	}
	h := &OTLPRelayHandler{
		endpoint: strings.TrimRight(endpoint, "/"),
		headers:  parseOTLPHeaders(headers),
		enabled:  enabled,
		client:   &http.Client{},
	}
	return h
}

// RelayTraces accepts OTLP HTTP/JSON from the browser and forwards it upstream.
func (h *OTLPRelayHandler) RelayTraces(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxOTLPBodySize+1))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if len(body) > maxOTLPBodySize {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	url := h.endpoint + "/v1/traces"
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		slog.Error("otlp relay: failed to create upstream request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	upstream.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	for k, v := range h.headers {
		upstream.Header.Set(k, v)
	}

	resp, err := h.client.Do(upstream)
	if err != nil {
		slog.Error("otlp relay: upstream request failed", "error", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Error("otlp relay: failed to copy upstream response", "error", err)
	}
}

// parseOTLPHeaders parses the standard OTel header format "key=value,key2=value2".
func parseOTLPHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	if raw == "" {
		return headers
	}
	for _, pair := range strings.Split(raw, ",") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			slog.Warn(fmt.Sprintf("otlp relay: ignoring malformed header %q", pair))
			continue
		}
		headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return headers
}
