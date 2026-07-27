package pages

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"
)

func cellLang(cellType string) string {
	if cellType == "markdown" {
		return "markdown"
	}
	return "python"
}

func cmHostClass(cellType string) string {
	if cellType == "markdown" {
		return "cm-host is-md-edit"
	}
	return "cm-host"
}

// writeComponent wraps a write-only render function as a templ.Component.
func writeComponent(fn func(w io.Writer) error) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return fn(w)
	})
}

// cellSourceJSON emits a JSON script tag. json must be encoding/json output.
func cellSourceJSON(cellID, json string) templ.Component {
	return writeComponent(func(w io.Writer) error {
		// cell IDs are generated UUIDs / internal ids — still attribute-escape.
		_, err := fmt.Fprintf(w,
			`<script type="application/json" class="cell-source-json" data-cell-id="%s">%s</script>`,
			templ.EscapeString(cellID), json)
		return err
	})
}

// gadernoBoot emits window.__GADERNO__ from json.Marshal'd path and kernel.
func gadernoBoot(pathJSON, kernelJSON string) templ.Component {
	return writeComponent(func(w io.Writer) error {
		_, err := fmt.Fprintf(w,
			"<script>\n\twindow.__GADERNO__ = {\n\t\tpath: %s,\n\t\tkernel: %s\n\t};\n</script>\n",
			pathJSON, kernelJSON)
		return err
	})
}
