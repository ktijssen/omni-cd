package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON serialises v as JSON, sets Content-Type, writes the HTTP status,
// and logs (but does not propagate) any encoding error. The header is
// already partially flushed by the time an Encode error fires, so the
// caller has no useful recovery path — surface it in logs instead of
// silently dropping it.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to write JSON response", "error", err, "component", "Web")
	}
}
