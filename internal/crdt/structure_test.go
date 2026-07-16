package crdt

import (
	"testing"

	"github.com/lucasew/gaderno/internal/document"
)

func TestInsertMoveDelete(t *testing.T) {
	d := New()
	nb := document.NewEmpty()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	ids := d.CellIDs()
	if len(ids) != 1 {
		t.Fatalf("want 1 got %v", ids)
	}
	id2, err := d.InsertCell(1, document.CellMarkdown, "# hi")
	if err != nil {
		t.Fatal(err)
	}
	ids = d.CellIDs()
	if len(ids) != 2 || ids[1] != id2 {
		t.Fatalf("%v", ids)
	}
	if err := d.MoveCell(id2, 0); err != nil {
		t.Fatal(err)
	}
	ids = d.CellIDs()
	if ids[0] != id2 {
		t.Fatalf("move %v", ids)
	}
	if err := d.DeleteCell(id2); err != nil {
		t.Fatal(err)
	}
	ids = d.CellIDs()
	if len(ids) != 1 {
		t.Fatalf("after delete %v", ids)
	}
	// ProjectNotebook must not invent cells
	proj := d.ProjectNotebook()
	if len(proj.Cells) != 1 {
		t.Fatalf("project %d", len(proj.Cells))
	}
}

func TestEnsureUniqueIDs(t *testing.T) {
	nb := &document.Notebook{
		NBFormat: 4,
		Cells: []document.Cell{
			{ID: "same", CellType: document.CellCode, Source: document.NewMultiline("a")},
			{ID: "same", CellType: document.CellCode, Source: document.NewMultiline("b")},
		},
	}
	document.EnsureCellIDs(nb)
	if nb.Cells[0].ID == nb.Cells[1].ID {
		t.Fatal("duplicate ids not fixed")
	}
}
