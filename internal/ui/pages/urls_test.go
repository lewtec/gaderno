package pages

import (
	"strings"
	"testing"
)

func TestEscapeNotebookPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"plain.ipynb", "plain.ipynb"},
		{"My Notebook.ipynb", "My%20Notebook.ipynb"},
		{"sub/My Notebook.ipynb", "sub/My%20Notebook.ipynb"},
		{"a+b.ipynb", "a+b.ipynb"}, // PathEscape leaves + alone
		{"100%.ipynb", "100%25.ipynb"},
	}
	for _, tc := range tests {
		if got := escapeNotebookPath(tc.in); got != tc.want {
			t.Errorf("escapeNotebookPath(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNotebookOpenURLEncodesSpaces(t *testing.T) {
	u := string(notebookOpenURL("My Notebook.ipynb"))
	if !strings.Contains(u, "My%20Notebook.ipynb") {
		t.Fatalf("open URL missing encoded space: %q", u)
	}
	if strings.Contains(u, "My Notebook") {
		t.Fatalf("open URL still has raw space: %q", u)
	}
}

func TestNotebookExportURLEncodesSpaces(t *testing.T) {
	u := string(notebookExportURL("dir/My Notebook.ipynb"))
	if !strings.HasPrefix(u, "/api/notebooks/dir/My%20Notebook.ipynb") {
		t.Fatalf("export URL: %q", u)
	}
	if !strings.HasSuffix(u, "?download=1") {
		t.Fatalf("export URL missing download query: %q", u)
	}
}
