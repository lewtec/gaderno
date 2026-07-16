package crdt

import (
	"testing"

	"github.com/lucasew/gaderno/internal/document"
)

func TestLoadProjectRoundTrip(t *testing.T) {
	nb := document.NewEmpty()
	nb.Cells[0].Source = document.NewMultiline("print(42)")
	d := New()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	ids := d.CellIDs()
	if len(ids) != 1 {
		t.Fatalf("ids %v", ids)
	}
	if got := d.Source(ids[0]); got != "print(42)" {
		t.Fatalf("source %q", got)
	}
	out := d.ProjectNotebook()
	if out.Cells[0].SourceString() != "print(42)" {
		t.Fatalf("project %q", out.Cells[0].SourceString())
	}
}

func TestSetSourceServer(t *testing.T) {
	nb := document.NewEmpty()
	d := New()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	id := d.CellIDs()[0]
	if err := d.SetSourceServer(id, "hello"); err != nil {
		t.Fatal(err)
	}
	if d.Source(id) != "hello" {
		t.Fatalf("source %q", d.Source(id))
	}
}
