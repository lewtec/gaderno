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

// cellJSONScript emits a JSON script tag. json must be encoding/json output.
func cellJSONScript(class, cellID, json string) templ.Component {
	return writeComponent(func(w io.Writer) error {
		// cell IDs are generated UUIDs / internal ids — still attribute-escape.
		_, err := fmt.Fprintf(w,
			`<script type="application/json" class="%s" data-cell-id="%s">%s</script>`,
			class, templ.EscapeString(cellID), json)
		return err
	})
}

// cellSourceJSON emits a JSON script tag. json must be encoding/json output.
func cellSourceJSON(cellID, json string) templ.Component {
	return cellJSONScript("cell-source-json", cellID, json)
}

// cellResultJSON emits saved outputs + execution_count for first paint.
func cellResultJSON(cellID, json string) templ.Component {
	return cellJSONScript("cell-result-json", cellID, json)
}

// gadernoBoot emits window.__GADERNO__ from json.Marshal'd path and kernel.
// agentBoot emits window.__GADERNO_AGENT__ from json.Marshal'd token.
func agentBoot(tokenJSON string) templ.Component {
	return writeComponent(func(w io.Writer) error {
		_, err := fmt.Fprintf(w,
			"<script>\nwindow.__GADERNO_AGENT__ = { token: %s };\n</script>\n",
			tokenJSON)
		return err
	})
}

func gadernoBoot(pathJSON, kernelJSON string) templ.Component {
	return writeComponent(func(w io.Writer) error {
		_, err := fmt.Fprintf(w,
			"<script>\n\twindow.__GADERNO__ = {\n\t\tpath: %s,\n\t\tkernel: %s\n\t};\n</script>\n",
			pathJSON, kernelJSON)
		return err
	})
}
