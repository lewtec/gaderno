package pages

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorkspaceRender(t *testing.T) {
	var buf bytes.Buffer
	if err := Workspace(WorkspaceData{Notebooks: []string{"a.ipynb"}}).Render(t.Context(), &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, s := range []string{"/static/vendor/daisyui.css", "/static/vendor/tailwind-browser.js", "/static/favicon.svg", "gaderno-light", "a.ipynb", "g-nb-row"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q", s)
		}
	}
}

func TestNotebookRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Notebook(NotebookData{
		Path:       "demo.ipynb",
		PathJSON:   `"demo.ipynb"`,
		KernelJSON: `"python3"`,
		Cells: []CellView{{
			Type:       "code",
			ID:         "c1",
			SourceJSON: `"print(1)"`,
			ResultJSON: `{"outputs":[{"output_type":"stream","name":"stdout","text":"1\n"}],"execution_count":1}`,
		}},
	}).Render(t.Context(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, s := range []string{`window.__GADERNO__`, `"demo.ipynb"`, `data-cell-id="c1"`, `cell-source-json`, `cell-result-json`, `/static/app.js`, `"python3"`, `print(1)`, `"stdout"`, `id="chat-toasts"`, `toast toast-end toast-bottom`} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	idx := strings.Index(out, "window.__GADERNO__")
	if idx >= 0 {
		end := idx + 180
		if end > len(out) {
			end = len(out)
		}
		t.Logf("boot: %s", out[idx:end])
	}
	idx = strings.Index(out, "cell-source-json")
	if idx >= 0 {
		end := idx + 120
		if end > len(out) {
			end = len(out)
		}
		t.Logf("cell: %s", out[idx:end])
	}
}
