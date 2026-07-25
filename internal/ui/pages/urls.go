package pages

import "github.com/a-h/templ"

func notebookOpenURL(name string) templ.SafeURL {
	return templ.URL("/n/" + name)
}

func notebookExportURL(path string) templ.SafeURL {
	return templ.URL("/api/notebooks/" + path + "?download=1")
}
