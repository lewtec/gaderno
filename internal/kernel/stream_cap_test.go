package kernel

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateStreamUnderLimit(t *testing.T) {
	s := "hello"
	if got := truncateStream(s, MaxStreamBytes); got != s {
		t.Fatalf("got %q", got)
	}
	if got := capStream(s); got != s {
		t.Fatalf("capStream %q", got)
	}
}

func TestTruncateStreamOverLimit(t *testing.T) {
	// Small max so the test stays light; still exercises cut + notice.
	const max = 64
	body := strings.Repeat("x", 200)
	got := truncateStream(body, max)
	if len(got) > max {
		t.Fatalf("len=%d > max=%d", len(got), max)
	}
	if !strings.Contains(got, "truncated stream output") {
		t.Fatalf("missing notice: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatal("invalid utf8 after truncate")
	}
}

func TestTruncateStreamUTF8Boundary(t *testing.T) {
	// "é" is 2 bytes; force a cut that would land mid-rune without RuneStart.
	s := "aa" + strings.Repeat("é", 40)
	const max = 20
	got := truncateStream(s, max)
	if len(got) > max {
		t.Fatalf("len=%d > max=%d got=%q", len(got), max, got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("invalid utf8: %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("missing notice: %q", got)
	}
}

func TestCapStreamUsesMaxStreamBytes(t *testing.T) {
	// Avoid allocating 12MiB+ in every CI run: temporarily cannot change const,
	// so only assert the helper path via truncateStream + that MaxStreamBytes
	// matches display order of magnitude (same hard-cap policy).
	if MaxStreamBytes != MaxDisplayBytes {
		t.Fatalf("MaxStreamBytes=%d MaxDisplayBytes=%d (expect same v1 cap)", MaxStreamBytes, MaxDisplayBytes)
	}
	if MaxStreamBytes < 1<<20 {
		t.Fatalf("MaxStreamBytes too small: %d", MaxStreamBytes)
	}
	// Over limit with a modest buffer relative to MaxStreamBytes would OOM CI;
	// unit coverage of the cut lives in TestTruncateStreamOverLimit.
	small := strings.Repeat("y", 100)
	if got := capStream(small); got != small {
		t.Fatalf("small stream changed: %q", got)
	}
}
