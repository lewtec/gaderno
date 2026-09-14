package app

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

func renderTempl(w http.ResponseWriter, r *http.Request, logger *slog.Logger, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		logger.Error("render templ", "err", err, "path", r.URL.Path)
	}
}
