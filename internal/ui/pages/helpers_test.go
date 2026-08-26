package pages

import (
	"bytes"
	"testing"

	"github.com/a-h/templ"
)

func TestCellJSONScripts(t *testing.T) {
	tests := []struct {
		name  string
		fn    func(cellID, json string) templ.Component
		class string
	}{
		{"source", cellSourceJSON, "cell-source-json"},
		{"result", cellResultJSON, "cell-result-json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tt.fn(`c"1`, `{"ok":true}`).Render(t.Context(), &buf); err != nil {
				t.Fatal(err)
			}
			got := buf.String()
			want := `<script type="application/json" class="` + tt.class + `" data-cell-id="` + templ.EscapeString(`c"1`) + `">{"ok":true}</script>`
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
