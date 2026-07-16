package app

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lucasew/gaderno/internal/workspace"
)

var listPage = template.Must(template.New("list").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>gaderno</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 2rem; max-width: 48rem; }
    h1 { font-size: 1.25rem; }
    ul { padding-left: 1.25rem; }
    a { color: #06c; }
    .empty { color: #666; }
    form { margin-top: 1.5rem; display: flex; gap: 0.5rem; flex-wrap: wrap; }
    input[type=text] { flex: 1; min-width: 12rem; padding: 0.4rem 0.5rem; }
    button { padding: 0.4rem 0.75rem; cursor: pointer; }
  </style>
</head>
<body>
  <h1>gaderno</h1>
  <p>Workspace notebooks</p>
  {{ if .Notebooks }}
  <ul>
    {{ range .Notebooks }}
    <li><a href="/n/{{ . }}">{{ . }}</a></li>
    {{ end }}
  </ul>
  {{ else }}
  <p class="empty">No notebooks yet.</p>
  {{ end }}
  <form method="post" action="/api/notebooks">
    <input type="text" name="name" placeholder="name.ipynb" required aria-label="Notebook name">
    <button type="submit">Create</button>
  </form>
</body>
</html>`))

func registerWorkspaceRoutes(mux *http.ServeMux, ws *workspace.Workspace, logger *slog.Logger) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		list, err := ws.List()
		if err != nil {
			logger.Error("list notebooks", "err", err)
			http.Error(w, "list failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := listPage.Execute(w, map[string]any{"Notebooks": list}); err != nil {
			logger.Error("render list", "err", err)
		}
	})

	mux.HandleFunc("GET /api/notebooks", func(w http.ResponseWriter, r *http.Request) {
		list, err := ws.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"notebooks": list})
	})

	mux.HandleFunc("POST /api/notebooks", func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		if name == "" && r.Header.Get("Content-Type") == "application/json" {
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				name = body.Name
			}
		}
		name = strings.TrimSpace(name)
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		path, err := ws.Create(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if wantsHTML(r) {
			http.Redirect(w, r, "/n/"+path, http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"path": path})
	})
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") || r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" || r.FormValue("name") != ""
}
