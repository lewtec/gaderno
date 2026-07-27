package app

import (
	"encoding/json"
	"net/http"
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
