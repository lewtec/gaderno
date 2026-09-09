package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lucasew/gaderno/internal/document"
	"github.com/lucasew/gaderno/internal/session"
	"github.com/lucasew/gaderno/internal/store"
	"github.com/lucasew/gaderno/internal/workspace"
)

func newAgentMux(t *testing.T) (*http.ServeMux, *session.Registry, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st := store.New(dir)
	ws := workspace.New(dir)
	reg := session.NewRegistry(st, dir)
	t.Cleanup(func() { reg.CloseAll(t.Context()) })
	mux := http.NewServeMux()
	registerWorkspaceRoutes(mux, ws, slog.Default())
	registerNotebookRoutes(mux, st, reg, "python3", slog.Default())
	registerKernelRoutes(mux, reg, slog.Default())
	registerAgentRoutes(mux, reg)
	return mux, reg, st
}

func doJSON(t *testing.T, mux http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode %s: %v\nbody: %s", rec.Result().Status, err, rec.Body.String())
	}
	return v
}

func openSessionByPath(t *testing.T, mux http.Handler, path string) agentNotebook {
	t.Helper()
	rec := doJSON(t, mux, http.MethodPost, "/api/sessions", map[string]any{"path": path})
	if rec.Code != http.StatusOK {
		t.Fatalf("open session %s: status %d body %s", path, rec.Code, rec.Body.String())
	}
	nb := decodeBody[agentNotebook](t, rec)
	if nb.SessionID == "" {
		t.Fatal("missing session_id")
	}
	if nb.Path != path {
		t.Fatalf("path %q want %q", nb.Path, path)
	}
	return nb
}

func TestAgentContractRoute(t *testing.T) {
	mux, _, _ := newAgentMux(t)
	rec := doJSON(t, mux, http.MethodGet, "/api/agent", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agent status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Fatalf("Content-Type=%q", ct)
	}
	body := rec.Body.String()
	for _, needle := range []string{"/api/sessions/$SID/cells", "/api/sessions/$SID", "session_id"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("contract missing %q", needle)
		}
	}
	if strings.Contains(body, "/api/sessions/$SID/kernel") {
		t.Fatal("contract must not offer kernel bind")
	}
}

func TestOpenSessionJoinsSameHub(t *testing.T) {
	mux, reg, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	a := openSessionByPath(t, mux, "n.ipynb")
	b := openSessionByPath(t, mux, "n.ipynb")
	if a.SessionID != b.SessionID {
		t.Fatalf("re-open minted a new session: %s vs %s", a.SessionID, b.SessionID)
	}
	hub, err := reg.GetByID(a.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if hub.Path != "n.ipynb" {
		t.Fatalf("hub path %q", hub.Path)
	}

	rec := doJSON(t, mux, http.MethodGet, "/api/sessions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	var listed struct {
		Sessions []sessionRow `json:"sessions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != a.SessionID || listed.Sessions[0].Path != "n.ipynb" {
		t.Fatalf("list %+v", listed.Sessions)
	}
}

func TestCompactViewOmitsBinaryMimes(t *testing.T) {
	mux, reg, st := newAgentMux(t)
	nb := document.NewEmpty()
	if err := st.Save(t.Context(), "n.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	hub, err := reg.GetOrOpen(t.Context(), "n.ipynb")
	if err != nil {
		t.Fatal(err)
	}
	id := hub.Doc.CellIDs()[0]
	count := 3
	if err := hub.Doc.ApplyCellExecution(id, []document.Output{
		{OutputType: "stream", Name: "stdout", Text: document.NewMultiline("hello\n")},
		{OutputType: "stream", Name: "stderr", Text: document.NewMultiline("warn\n")},
		{OutputType: "display_data", Data: map[string]any{
			"image/png":  "AAAA",
			"text/html":  "<b>x</b>",
			"text/plain": "<Figure>",
		}},
		{OutputType: "error", Ename: "ValueError", Evalue: "nope"},
	}, &count, "error"); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, mux, http.MethodGet, "/api/sessions/"+hub.SessionID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "AAAA") {
		t.Fatal("compact view leaked image payload")
	}
	got := decodeBody[agentNotebook](t, rec)
	if got.SessionID != hub.SessionID {
		t.Fatalf("session_id %q", got.SessionID)
	}
	if got.Path != "n.ipynb" {
		t.Fatalf("path %q", got.Path)
	}
	if len(got.Cells) != 1 {
		t.Fatalf("cells=%d", len(got.Cells))
	}
	c := got.Cells[0]
	if c.ID != id {
		t.Fatalf("id %q want %q", c.ID, id)
	}
	if c.Stdout != "hello\n" {
		t.Fatalf("stdout %q", c.Stdout)
	}
	if c.Stderr != "warn\n" {
		t.Fatalf("stderr %q", c.Stderr)
	}
	if c.Error != "ValueError: nope" {
		t.Fatalf("error %q", c.Error)
	}
	if c.Text != "<Figure>" {
		t.Fatalf("text %q", c.Text)
	}
	if diff := cmp.Diff([]string{"image/png", "text/html"}, c.Omitted); diff != "" {
		t.Fatalf("omitted mismatch (-want +got):\n%s", diff)
	}
}

func TestCellMutationsRoundTrip(t *testing.T) {
	mux, _, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	opened := openSessionByPath(t, mux, "n.ipynb")
	sid := opened.SessionID
	base := "/api/sessions/" + sid

	rec := doJSON(t, mux, http.MethodPost, base+"/cells", map[string]any{
		"index":  1,
		"type":   "markdown",
		"source": "# hi",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("insert status %d body %s", rec.Code, rec.Body.String())
	}
	nb := decodeBody[agentNotebook](t, rec)
	if len(nb.Cells) != 2 {
		t.Fatalf("after insert cells=%d", len(nb.Cells))
	}
	mdID := nb.Cells[1].ID
	if nb.Cells[1].Type != "markdown" || nb.Cells[1].Source != "# hi" {
		t.Fatalf("inserted cell %+v", nb.Cells[1])
	}

	rec = doJSON(t, mux, http.MethodPatch, base+"/cells/"+mdID, map[string]any{
		"source": "# hello",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("source status %d body %s", rec.Code, rec.Body.String())
	}
	nb = decodeBody[agentNotebook](t, rec)
	if nb.Cells[1].Source != "# hello" {
		t.Fatalf("source %q", nb.Cells[1].Source)
	}

	rec = doJSON(t, mux, http.MethodPatch, base+"/cells/"+mdID, map[string]any{
		"type": "code",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("type status %d body %s", rec.Code, rec.Body.String())
	}
	nb = decodeBody[agentNotebook](t, rec)
	if nb.Cells[1].Type != "code" {
		t.Fatalf("type %q", nb.Cells[1].Type)
	}

	rec = doJSON(t, mux, http.MethodPost, base+"/cells/"+mdID+"/move", map[string]any{
		"index": 0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("move status %d body %s", rec.Code, rec.Body.String())
	}
	nb = decodeBody[agentNotebook](t, rec)
	if nb.Cells[0].ID != mdID {
		t.Fatalf("move order %q %q", nb.Cells[0].ID, nb.Cells[1].ID)
	}

	rec = doJSON(t, mux, http.MethodDelete, base+"/cells/"+mdID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status %d body %s", rec.Code, rec.Body.String())
	}
	nb = decodeBody[agentNotebook](t, rec)
	if len(nb.Cells) != 1 {
		t.Fatalf("after delete cells=%d", len(nb.Cells))
	}
	if nb.Cells[0].ID == mdID {
		t.Fatal("deleted cell still present")
	}
}

func TestCellMutationErrors(t *testing.T) {
	mux, _, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	sid := openSessionByPath(t, mux, "n.ipynb").SessionID
	base := "/api/sessions/" + sid

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
	}{
		{
			name:   "unknown session",
			method: http.MethodPost,
			path:   "/api/sessions/not-a-session/cells",
			body:   map[string]any{"source": "x"},
			status: http.StatusNotFound,
		},
		{
			name:   "unknown cell",
			method: http.MethodPatch,
			path:   base + "/cells/deadbeef",
			body:   map[string]any{"source": "x"},
			status: http.StatusNotFound,
		},
		{
			name:   "invalid type",
			method: http.MethodPost,
			path:   base + "/cells",
			body:   map[string]any{"type": "sql", "source": "x"},
			status: http.StatusBadRequest,
		},
		{
			name:   "missing notebook on open",
			method: http.MethodPost,
			path:   "/api/sessions",
			body:   map[string]any{"path": "nope.ipynb"},
			status: http.StatusNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, mux, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status %d want %d body %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestExecuteWritesSourceBeforeKernel(t *testing.T) {
	mux, reg, st := newAgentMux(t)
	nb := document.NewEmpty()
	nb.Metadata["kernelspec"] = map[string]any{
		"name":         "gaderno-test-missing",
		"display_name": "missing",
	}
	if err := st.Save(t.Context(), "n.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	opened := openSessionByPath(t, mux, "n.ipynb")
	id := opened.Cells[0].ID

	rec := doJSON(t, mux, http.MethodPost, "/api/sessions/"+opened.SessionID+"/cells/"+id+"/execute", map[string]any{
		"source": "print(9)",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("execute status %d want 400 body %s", rec.Code, rec.Body.String())
	}
	hub, err := reg.GetByID(opened.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if src := hub.Doc.Source(id); src != "print(9)" {
		t.Fatalf("source after failed execute %q", src)
	}
}

func TestExecuteIgnoresKernelField(t *testing.T) {
	mux, reg, st := newAgentMux(t)
	nb := document.NewEmpty()
	nb.Metadata["kernelspec"] = map[string]any{
		"name":         "gaderno-test-missing",
		"display_name": "missing",
	}
	if err := st.Save(t.Context(), "n.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	opened := openSessionByPath(t, mux, "n.ipynb")
	id := opened.Cells[0].ID

	// A kernel name in the body must not bind. Missing spec would be 409 if
	// BindKernel ran; no bound spec is 400.
	rec := doJSON(t, mux, http.MethodPost, "/api/sessions/"+opened.SessionID+"/cells/"+id+"/execute", map[string]any{
		"kernel": "gaderno-test-missing",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("execute status %d want 400 body %s", rec.Code, rec.Body.String())
	}
	hub, err := reg.GetByID(opened.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	stt := hub.Status()
	if !stt.NeedsPick || stt.BoundName != "" {
		t.Fatalf("kernel was bound from execute body: %+v", stt)
	}
}

func TestAgentKernelBindRouteGone(t *testing.T) {
	mux, _, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	sid := openSessionByPath(t, mux, "n.ipynb").SessionID
	rec := doJSON(t, mux, http.MethodPost, "/api/sessions/"+sid+"/kernel", map[string]any{
		"name": "python3",
	})
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("kernel bind status %d want 404/405 body %s", rec.Code, rec.Body.String())
	}
}

func TestInterruptWithoutKernel(t *testing.T) {
	mux, _, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	sid := openSessionByPath(t, mux, "n.ipynb").SessionID
	rec := doJSON(t, mux, http.MethodPost, "/api/sessions/"+sid+"/interrupt", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d want 409 body %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownSession(t *testing.T) {
	mux, _, _ := newAgentMux(t)
	rec := doJSON(t, mux, http.MethodGet, "/api/sessions/missing", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d want 404 body %s", rec.Code, rec.Body.String())
	}
}

func TestCompactViewDefaultIsFullNotebook(t *testing.T) {
	mux, _, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, mux, http.MethodGet, "/api/notebooks/n.ipynb", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var nb document.Notebook
	if err := json.NewDecoder(rec.Body).Decode(&nb); err != nil {
		t.Fatal(err)
	}
	if nb.NBFormat != 4 {
		t.Fatalf("expected full nbformat, got %+v", nb)
	}
}

func TestSessionChatHTTP(t *testing.T) {
	mux, _, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	sid := openSessionByPath(t, mux, "n.ipynb").SessionID
	base := "/api/sessions/" + sid + "/chat"

	rec := doJSON(t, mux, http.MethodPost, base, map[string]any{"text": "  hi there  "})
	if rec.Code != http.StatusOK {
		t.Fatalf("post status %d body %s", rec.Code, rec.Body.String())
	}
	msg := decodeBody[session.ChatMessage](t, rec)
	if msg.From != session.ChatFromAgent || msg.Text != "hi there" {
		t.Fatalf("%+v", msg)
	}

	rec = doJSON(t, mux, http.MethodPost, base, map[string]any{"text": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty status %d", rec.Code)
	}

	rec = doJSON(t, mux, http.MethodGet, base, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d", rec.Code)
	}
	var got struct {
		Messages []session.ChatMessage `json:"messages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Text != "hi there" {
		t.Fatalf("%+v", got.Messages)
	}
}

func TestGetByIDAfterCloseAll(t *testing.T) {
	_, reg, st := newAgentMux(t)
	if err := st.Save(t.Context(), "n.ipynb", document.NewEmpty()); err != nil {
		t.Fatal(err)
	}
	hub, err := reg.GetOrOpen(t.Context(), "n.ipynb")
	if err != nil {
		t.Fatal(err)
	}
	id := hub.SessionID
	reg.CloseAll(t.Context())
	if _, err := reg.GetByID(id); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("after CloseAll GetByID: %v", err)
	}
}
