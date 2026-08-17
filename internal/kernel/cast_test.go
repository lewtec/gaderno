package kernel

import (
	"strings"
	"testing"
)

func TestParseCastV2(t *testing.T) {
	const src = `{"version":2,"width":80,"height":24}
[0.0, "o", "hello"]
[0.1, "i", "ignored"]
[0.2, "o", "\rworld\n"]
`
	cols, rows, chunks, err := parseCast(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if cols != 80 || rows != 24 {
		t.Fatalf("size %dx%d", cols, rows)
	}
	if len(chunks) != 2 || chunks[0] != "hello" || chunks[1] != "\rworld\n" {
		t.Fatalf("chunks %#v", chunks)
	}
	got := replayCast(cols, rows, chunks)
	if got != "world" {
		t.Fatalf("replay %q", got)
	}
}

func TestParseCastV3(t *testing.T) {
	const src = `{"version":3,"term":{"cols":40,"rows":12}}
[0.0, "o", "x"]
# comment
[0.1, "r", "80x24"]
[0.2, "o", "y"]
`
	cols, rows, chunks, err := parseCast(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if cols != 40 || rows != 12 {
		t.Fatalf("size %dx%d", cols, rows)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks %#v", chunks)
	}
}
