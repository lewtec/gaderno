package kernel

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var updateTermout = flag.Bool("update", false, "rewrite testdata/termout/*.want")

func TestFilterTerminal(t *testing.T) {
	root := filepath.Join("testdata", "termout")
	casts, err := filepath.Glob(filepath.Join(root, "*.cast"))
	if err != nil {
		t.Fatal(err)
	}
	if len(casts) == 0 {
		t.Fatalf("no *.cast under %s", root)
	}
	for _, castPath := range casts {
		name := strings.TrimSuffix(filepath.Base(castPath), ".cast")
		t.Run(name, func(t *testing.T) {
			cols, rows, chunks, err := parseCastFile(castPath)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.TrimRight(replayCast(cols, rows, chunks), "\n") + "\n"
			wantPath := filepath.Join(root, name+".want")
			if *updateTermout {
				if err := os.WriteFile(wantPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			wantb, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimRight(string(wantb), "\n") + "\n"
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTermFilter_csiCursorDownDoesNotAllocateHuge(t *testing.T) {
	var f TermFilter
	f.Write("\x1b[999999999Bboom")
	got := f.String()
	if !strings.Contains(got, "boom") {
		t.Fatalf("expected content after bounded move: %q", got)
	}
}

func TestTermFilter_csiCursorForwardDoesNotPadHuge(t *testing.T) {
	var f TermFilter
	f.Write("\x1b[999999999Cx")
	got := f.String()
	if !strings.Contains(got, "x") {
		t.Fatalf("expected glyph after bounded move: %q", got)
	}
}

func TestTermFilter_scrollsWhenExceedingViewport(t *testing.T) {
	f := NewTermFilter(80, 24)
	const extra = 10
	for i := 0; i < 24+extra; i++ {
		f.Write("L" + strings.Repeat("x", 3) + "\n")
	}
	got := f.String()
	if !strings.Contains(got, "Lxxx") {
		t.Fatalf("expected scrolled content: %q", got)
	}
}
