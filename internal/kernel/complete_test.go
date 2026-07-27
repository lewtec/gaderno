package kernel

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompleteNoConnection(t *testing.T) {
	m := &Manager{}
	_, err := m.Complete(t.Context(), "import os", 0)
	if !errors.Is(err, ErrNoConnection) {
		t.Fatalf("errors.Is(ErrNoConnection)=false: %v", err)
	}
}

func TestParseCompleteReply(t *testing.T) {
	res := parseCompleteReply(map[string]any{
		"status":       "ok",
		"matches":      []any{"os.path", "os.environ"},
		"cursor_start": 3.0,
		"cursor_end":   5.0,
	}, 5)
	if res.Status != "ok" {
		t.Fatalf("status %q", res.Status)
	}
	if res.CursorStart != 3 || res.CursorEnd != 5 {
		t.Fatalf("cursors %d %d", res.CursorStart, res.CursorEnd)
	}
	if len(res.Matches) != 2 || res.Matches[0] != "os.path" {
		t.Fatalf("matches %#v", res.Matches)
	}
	empty := parseCompleteReply(nil, 1)
	if empty.CursorStart != 1 || empty.Matches == nil {
		t.Fatalf("empty %#v", empty)
	}
}

func TestParseCompleteReplyCapsMatches(t *testing.T) {
	matches := make([]any, MaxCompleteMatches+200)
	for i := range matches {
		matches[i] = "m" + strings.Repeat("x", 8)
	}
	// One oversize match should be truncated, not dropped.
	matches[0] = strings.Repeat("a", MaxCompleteMatchBytes+40)

	res := parseCompleteReply(map[string]any{
		"status":  "ok",
		"matches": matches,
	}, 0)
	if len(res.Matches) != MaxCompleteMatches {
		t.Fatalf("got %d matches, want %d", len(res.Matches), MaxCompleteMatches)
	}
	if len(res.Matches[0]) > MaxCompleteMatchBytes {
		t.Fatalf("match[0] len %d > cap %d", len(res.Matches[0]), MaxCompleteMatchBytes)
	}
	if !utf8.ValidString(res.Matches[0]) {
		t.Fatalf("truncated match is invalid UTF-8")
	}
}

func TestClampCompleteCodeWindow(t *testing.T) {
	// Cursor near the end of a huge cell: keep a MaxCompleteCodeBytes window.
	prefix := strings.Repeat("x", MaxCompleteCodeBytes+100)
	suffix := "import os"
	code := prefix + suffix
	cursor := len(code)
	got, pos := clampCompleteCode(code, cursor)
	if len(got) > MaxCompleteCodeBytes {
		t.Fatalf("code len %d > cap", len(got))
	}
	if !strings.HasSuffix(got, suffix) {
		t.Fatalf("lost cursor neighborhood: %q…", got[len(got)-20:])
	}
	if pos != len(got) {
		t.Fatalf("cursor pos %d want %d", pos, len(got))
	}
	// Negative cursor clamps.
	_, pos = clampCompleteCode("abc", -3)
	if pos != 0 {
		t.Fatalf("neg cursor → %d", pos)
	}
}
