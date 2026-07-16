package app

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/lucasew/gaderno/internal/document"
	"github.com/lucasew/gaderno/internal/store"
)

var notebookPage = template.Must(template.New("notebook").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Path }} — gaderno</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 1.5rem; max-width: 52rem; }
    a { color: #06c; }
    .cell { border: 1px solid #ddd; border-radius: 6px; margin: 0.75rem 0; padding: 0.75rem; }
    .meta { color: #666; font-size: 0.85rem; margin-bottom: 0.5rem; }
    pre { white-space: pre-wrap; margin: 0; font-family: ui-monospace, monospace; font-size: 0.9rem; }
    .out { background: #f7f7f7; margin-top: 0.5rem; padding: 0.5rem; border-radius: 4px; }
    .toolbar { margin-bottom: 1rem; display: flex; gap: 0.75rem; align-items: center; flex-wrap: wrap; }
  </style>
</head>
<body>
  <div class="toolbar">
    <a href="/">← Workspace</a>
    <strong>{{ .Path }}</strong>
    <a href="/api/notebooks/{{ .Path }}?download=1">Export .ipynb</a>
  </div>
  {{ range .Cells }}
  <div class="cell">
    <div class="meta">{{ .Type }}{{ if .ID }} · {{ .ID }}{{ end }}</div>
    <pre>{{ .Source }}</pre>
    {{ range .Outputs }}
    <div class="out"><pre>{{ . }}</pre></div>
    {{ end }}
  </div>
  {{ else }}
  <p>Empty notebook.</p>
  {{ end }}
</body>
</html>`))

func registerNotebookRoutes(mux *http.ServeMux, st *store.Store, logger *slog.Logger) {
	mux.HandleFunc("GET /n/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		nb, err := st.Load(r.Context(), path)
		if err != nil {
			if store.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			logger.Error("load notebook", "path", path, "err", err)
			http.Error(w, "load failed", http.StatusInternalServerError)
			return
		}
		type cellView struct {
			Type    string
			ID      string
			Source  string
			Outputs []string
		}
		var cells []cellView
		for _, c := range nb.Cells {
			cv := cellView{
				Type:   string(c.CellType),
				ID:     c.ID,
				Source: c.SourceString(),
			}
			for _, o := range c.Outputs {
				cv.Outputs = append(cv.Outputs, document.OutputPlain(o))
			}
			cells = append(cells, cv)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := notebookPage.Execute(w, map[string]any{"Path": path, "Cells": cells}); err != nil {
			logger.Error("render notebook", "err", err)
		}
	})

	mux.HandleFunc("GET /api/notebooks/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		nb, err := st.Load(r.Context(), path)
		if err != nil {
			if store.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("download") == "1" {
			raw, err := document.Encode(nb)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/x-ipynb+json")
			w.Header().Set("Content-Disposition", `attachment; filename="notebook.ipynb"`)
			_, _ = w.Write(raw)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nb)
	})
}
