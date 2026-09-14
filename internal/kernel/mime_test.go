package kernel

import (
	"strings"
	"testing"
)

func TestNormalizeMimeBundle(t *testing.T) {
	b := normalizeMimeBundle(map[string]any{
		"text/plain": []any{"hello", " ", "world"},
		"image/png":  "iVBORw0KGgo=",
	})
	if b["text/plain"] != "hello world" {
		t.Fatalf("plain %v", b["text/plain"])
	}
	if b["image/png"] != "iVBORw0KGgo=" {
		t.Fatalf("png %v", b["image/png"])
	}
	big := strings.Repeat("A", MaxDisplayBytes+10)
	b2 := normalizeMimeBundle(map[string]any{
		"image/png":  big,
		"text/plain": "fig",
	})
	if _, ok := b2["image/png"]; ok {
		t.Fatal("expected png dropped")
	}
	plain, _ := b2["text/plain"].(string)
	if !strings.Contains(plain, "omitted oversized") {
		t.Fatalf("plain note: %q", plain)
	}
}
