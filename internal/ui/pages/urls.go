package pages

import (
	"net/url"
	"strings"

	"github.com/a-h/templ"
)

// escapeNotebookPath percent-encodes each path segment so spaces and reserved
// characters survive in hrefs while "/" separators stay.
func escapeNotebookPath(path string) string {
	if path == "" {
		return path
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func notebookOpenURL(name string) templ.SafeURL {
	return templ.URL("/n/" + escapeNotebookPath(name))
}

func notebookExportURL(path string) templ.SafeURL {
	return templ.URL("/api/notebooks/" + escapeNotebookPath(path) + "?download=1")
}
