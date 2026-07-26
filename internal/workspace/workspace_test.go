package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAlreadyExistsWraps(t *testing.T) {
	dir := t.TempDir()
	w := New(dir)

	// seed an existing notebook under the jail
	path := filepath.Join(dir, "n.ipynb")
	if err := os.WriteFile(path, []byte(`{"nbformat":4,"nbformat_minor":5,"metadata":{},"cells":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := w.Create("n.ipynb")
	if err == nil {
		t.Fatal("expected error for existing notebook")
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("want errors.Is(err, fs.ErrExist), got %v", err)
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("want errors.Is(err, os.ErrExist), got %v", err)
	}
}
