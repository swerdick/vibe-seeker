package handlers

import (
	"net/http"

	"github.com/pseudo/vibe-seeker/backend/internal/observability"
)

// httpError logs an error with trace context and returns an HTTP error response.
func httpError(w http.ResponseWriter, r *http.Request, status int, userMsg string, logMsg string, args ...any) {
	observability.Logger(r.Context()).Error(logMsg, args...)
	http.Error(w, userMsg, status)
}
