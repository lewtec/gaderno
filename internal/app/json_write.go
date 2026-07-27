package app

import (
	"encoding/json"
	"net/http"

	"github.com/lucasew/gaderno/internal/session"
)

// writeJSON sets application/json and encodes v. Encode failures are ignored
// after headers may already be written (standard net/http JSON handlers).
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// client gone or broken pipe — nothing useful left to do
		return
	}
}

// openHub loads or attaches the hub for path. On failure it writes the HTTP
// error and returns ok=false.
func openHub(w http.ResponseWriter, r *http.Request, reg *session.Registry, path string) (*session.Hub, bool) {
	hub, err := reg.GetOrOpen(r.Context(), path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return nil, false
	}
	return hub, true
}
