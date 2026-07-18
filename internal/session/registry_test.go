package session

import (
	"context"
	"testing"

	"github.com/lucasew/gaderno/internal/document"
	"github.com/lucasew/gaderno/internal/store"
)

func TestGetOrOpenCanonicalPath(t *testing.T) {
	dir := t.TempDir()
	st := store.New(dir)
	nb := document.NewEmpty()
	if err := st.Save(context.Background(), "n.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(st, dir, "python3")
	defer reg.CloseAll(context.Background())

	h1, err := reg.GetOrOpen(context.Background(), "./n.ipynb")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := reg.GetOrOpen(context.Background(), "sub/../n.ipynb")
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("expected same hub for equivalent paths, got %p vs %p", h1, h2)
	}
	if h1.Path != "n.ipynb" {
		t.Fatalf("hub path = %q, want n.ipynb", h1.Path)
	}
	// Third spelling also hits the same hub (map key is CleanRel).
	h3, err := reg.GetOrOpen(context.Background(), "n.ipynb")
	if err != nil {
		t.Fatal(err)
	}
	if h3 != h1 {
		t.Fatalf("expected same hub for bare path")
	}
}

func TestGetOrOpenRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	st := store.New(dir)
	reg := NewRegistry(st, dir, "python3")
	for _, p := range []string{"", ".", ".."} {
		if _, err := reg.GetOrOpen(context.Background(), p); err == nil {
			t.Fatalf("GetOrOpen(%q) expected error", p)
		}
	}
}
