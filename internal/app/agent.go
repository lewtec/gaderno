package app

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/lucasew/gaderno/internal/crdt"
	"github.com/lucasew/gaderno/internal/document"
	"github.com/lucasew/gaderno/internal/session"
	"github.com/lucasew/gaderno/internal/store"
)

//go:embed agent.md
var agentContractFS embed.FS

var errTrailingJSON = errors.New("unexpected JSON after first value")

func registerAgentRoutes(mux *http.ServeMux, reg *session.Registry) {
	mux.HandleFunc("GET /api/agent", func(w http.ResponseWriter, r *http.Request) {
		raw, err := agentContractFS.ReadFile("agent.md")
		if err != nil {
			http.Error(w, "contract missing", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		if _, err := w.Write(raw); err != nil {
			return
		}
	})

	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		hubs := reg.Hubs()
		rows := make([]sessionRow, 0, len(hubs))
		for _, h := range hubs {
			rows = append(rows, sessionRow{
				ID:     h.SessionID,
				Path:   h.Path,
				Kernel: h.Status(),
			})
		}
		writeJSON(w, map[string]any{"sessions": rows})
	})

	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if err := decodeJSONBody(r, &body); err != nil || body.Path == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}
		hub, ok := openHub(w, r, reg, body.Path)
		if !ok {
			return
		}
		writeJSON(w, agentNotebookFromHub(hub))
	})

	mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		writeJSON(w, agentNotebookFromHub(hub))
	})

	mux.HandleFunc("POST /api/sessions/{id}/cells", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		var body struct {
			Index  *int   `json:"index"`
			Type   string `json:"type"`
			Source string `json:"source"`
		}
		if err := decodeJSONBodyOptional(r, &body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Type == "" {
			body.Type = string(document.CellCode)
		}
		if err := validateCellType(body.Type); err != nil {
			writeHubError(w, err)
			return
		}
		index := len(hub.Doc.CellIDs())
		if body.Index != nil {
			index = *body.Index
		}
		if _, err := hub.InsertCell(index, body.Type, body.Source); err != nil {
			writeHubError(w, err)
			return
		}
		writeJSON(w, agentNotebookFromHub(hub))
	})

	mux.HandleFunc("PATCH /api/sessions/{id}/cells/{cell}", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		cellID := r.PathValue("cell")
		if err := requireCell(hub, cellID); err != nil {
			writeHubError(w, err)
			return
		}
		var body struct {
			Source *string `json:"source"`
			Type   *string `json:"type"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Source == nil && body.Type == nil {
			http.Error(w, "source or type required", http.StatusBadRequest)
			return
		}
		if body.Type != nil {
			if err := hub.SetCellType(cellID, *body.Type); err != nil {
				writeHubError(w, err)
				return
			}
		}
		if body.Source != nil {
			if err := hub.SetCellSource(cellID, *body.Source, ""); err != nil {
				writeHubError(w, err)
				return
			}
		}
		writeJSON(w, agentNotebookFromHub(hub))
	})

	mux.HandleFunc("POST /api/sessions/{id}/cells/{cell}/move", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		var body struct {
			Index *int `json:"index"`
		}
		if err := decodeJSONBody(r, &body); err != nil || body.Index == nil {
			http.Error(w, "index required", http.StatusBadRequest)
			return
		}
		if err := hub.MoveCell(r.PathValue("cell"), *body.Index); err != nil {
			writeHubError(w, err)
			return
		}
		writeJSON(w, agentNotebookFromHub(hub))
	})

	mux.HandleFunc("DELETE /api/sessions/{id}/cells/{cell}", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		if err := hub.DeleteCell(r.PathValue("cell")); err != nil {
			writeHubError(w, err)
			return
		}
		writeJSON(w, agentNotebookFromHub(hub))
	})

	mux.HandleFunc("POST /api/sessions/{id}/cells/{cell}/execute", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		cellID := r.PathValue("cell")
		if err := requireCell(hub, cellID); err != nil {
			writeHubError(w, err)
			return
		}
		var body struct {
			Kernel string  `json:"kernel"`
			Source *string `json:"source"`
		}
		if err := decodeJSONBodyOptional(r, &body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Source != nil {
			if err := hub.SetCellSource(cellID, *body.Source, ""); err != nil {
				writeHubError(w, err)
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		if err := hub.EnsureKernel(ctx, body.Kernel); err != nil {
			writeEnsureKernelError(w, err)
			return
		}
		res, err := hub.ExecuteCell(ctx, cellID, nil, nil)
		if err != nil {
			writeHubError(w, err)
			return
		}
		writeJSON(w, res)
	})

	mux.HandleFunc("POST /api/sessions/{id}/interrupt", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		if err := hub.Interrupt(r.Context()); err != nil {
			writeHubError(w, err)
			return
		}
		writeJSON(w, hub.Status())
	})

	mux.HandleFunc("POST /api/sessions/{id}/kernel", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := decodeJSONBody(r, &body); err != nil || body.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := hub.BindKernel(body.Name); err != nil {
			writeHubError(w, err)
			return
		}
		writeJSON(w, hub.Status())
	})

	mux.HandleFunc("POST /api/sessions/{id}/save", func(w http.ResponseWriter, r *http.Request) {
		hub, ok := openSession(w, r, reg)
		if !ok {
			return
		}
		if err := hub.Save(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func openSession(w http.ResponseWriter, r *http.Request, reg *session.Registry) (*session.Hub, bool) {
	hub, err := reg.GetByID(r.PathValue("id"))
	if err != nil {
		writeHubError(w, err)
		return nil, false
	}
	return hub, true
}

type sessionRow struct {
	ID     string               `json:"id"`
	Path   string               `json:"path"`
	Kernel session.KernelStatus `json:"kernel"`
}

func decodeJSONBody(r *http.Request, v any) error {
	if err := decodeJSONBodyOptional(r, v); err != nil {
		return err
	}
	// Required body: treat a missing document as an error (caller checks fields).
	return nil
}

func decodeJSONBodyOptional(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errTrailingJSON
	}
	return nil
}

func validateCellType(cellType string) error {
	switch document.CellType(cellType) {
	case document.CellCode, document.CellMarkdown, document.CellRaw:
		return nil
	default:
		return session.ErrInvalidCellType
	}
}

func requireCell(hub *session.Hub, cellID string) error {
	if cellID == "" {
		return crdt.ErrEmptyCellID
	}
	for _, id := range hub.Doc.CellIDs() {
		if id == cellID {
			return nil
		}
	}
	return crdt.ErrCellNotFound
}

func writeHubError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, crdt.ErrCellNotFound),
		errors.Is(err, crdt.ErrEmptyCellID),
		errors.Is(err, session.ErrSessionNotFound),
		store.IsNotExist(err):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, session.ErrInvalidCellType),
		errors.Is(err, session.ErrKernelNameRequired),
		errors.Is(err, session.ErrNoKernelSelected):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, session.ErrKernelNotStarted),
		errors.Is(err, session.ErrKernelspecUnavailable):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, session.ErrKernelSpawnFailed),
		errors.Is(err, session.ErrKernelSpawnTimeout):
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeEnsureKernelError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrNoKernelSelected),
		errors.Is(err, session.ErrKernelNameRequired):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, session.ErrKernelspecUnavailable):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, "kernel: "+err.Error(), http.StatusBadGateway)
	}
}

// agentNotebook is the compact live projection for agents (no binary mimes).
type agentNotebook struct {
	SessionID string               `json:"session_id"`
	Path      string               `json:"path"`
	Kernel    session.KernelStatus `json:"kernel"`
	Cells     []agentCell          `json:"cells"`
}

type agentCell struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Source         string   `json:"source"`
	ExecutionCount *int     `json:"execution_count,omitempty"`
	Stdout         string   `json:"stdout,omitempty"`
	Stderr         string   `json:"stderr,omitempty"`
	Error          string   `json:"error,omitempty"`
	Text           string   `json:"text,omitempty"`
	Omitted        []string `json:"omitted,omitempty"`
}

func agentNotebookFromHub(hub *session.Hub) agentNotebook {
	out := agentNotebookFrom(hub.Path, hub.Doc.ProjectNotebook(), hub.Status())
	out.SessionID = hub.SessionID
	return out
}

func agentNotebookFrom(path string, nb *document.Notebook, st session.KernelStatus) agentNotebook {
	out := agentNotebook{
		Path:   path,
		Kernel: st,
		Cells:  []agentCell{},
	}
	if nb == nil {
		return out
	}
	for _, c := range nb.Cells {
		out.Cells = append(out.Cells, compactCell(c))
	}
	return out
}

func compactCell(c document.Cell) agentCell {
	ac := agentCell{
		ID:             c.ID,
		Type:           string(c.CellType),
		Source:         c.SourceString(),
		ExecutionCount: c.ExecutionCount,
	}
	if c.CellType != document.CellCode {
		return ac
	}
	seen := map[string]bool{}
	var omitted []string
	for _, o := range c.Outputs {
		switch o.OutputType {
		case "stream":
			text := ""
			if o.Text != nil {
				text = o.Text.String()
			}
			if o.Name == "stderr" {
				ac.Stderr += text
			} else {
				ac.Stdout += text
			}
		case "error":
			ac.Error = formatCellError(o.Ename, o.Evalue)
		case "execute_result", "display_data":
			if o.Data == nil {
				continue
			}
			if v, ok := o.Data["text/plain"]; ok {
				if s := mimeString(v); s != "" {
					if ac.Text != "" {
						ac.Text += "\n"
					}
					ac.Text += s
				}
			}
			for mime := range o.Data {
				if mime == "text/plain" || seen[mime] {
					continue
				}
				seen[mime] = true
				omitted = append(omitted, mime)
			}
		}
	}
	slices.Sort(omitted)
	ac.Omitted = omitted
	return ac
}

func formatCellError(ename, evalue string) string {
	switch {
	case ename != "" && evalue != "":
		return ename + ": " + evalue
	case ename != "":
		return ename
	default:
		return evalue
	}
}

func mimeString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []string:
		return strings.Join(t, "")
	case []any:
		var b strings.Builder
		for _, x := range t {
			b.WriteString(mimeString(x))
		}
		return b.String()
	default:
		return ""
	}
}
