package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lucasew/gaderno/internal/document"
)

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	nb := document.NewEmpty()
	nb.Cells[0].Source = document.NewMultiline("print(1)")
	if err := st.Save(nil, "a.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load(nil, "a.ipynb")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cells[0].SourceString() != "print(1)" {
		t.Fatalf("got %q", got.Cells[0].SourceString())
	}
}

func TestPathJail(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	_, err := st.Load(nil, "../etc/passwd")
	if err == nil {
		t.Fatal("expected escape error")
	}
}

func TestCreateNewExists(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	nb := document.NewEmpty()
	if err := st.CreateNew(nil, "x.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	err := st.CreateNew(nil, "x.ipynb", nb)
	if !os.IsExist(err) {
		t.Fatalf("want exist, got %v", err)
	}
	// file exists on disk
	if _, err := os.Stat(filepath.Join(dir, "x.ipynb")); err != nil {
		t.Fatal(err)
	}
}
