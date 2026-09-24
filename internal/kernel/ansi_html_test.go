package kernel

import (
	"strings"
	"testing"
)

func TestANSIToHTML_stripsAndColors(t *testing.T) {
	in := "\x1b[31mSignature:\x1b[39m os.chdir(path)\n\x1b[31mDocstring:\x1b[39m\nChange cwd"
	out := ANSIToHTML(in)
	if strings.Contains(out, "\x1b") || strings.Contains(out, "[31m") {
		t.Fatalf("raw ANSI leaked: %q", out)
	}
	if !strings.Contains(out, `class="ansi-fg-red"`) {
		t.Fatalf("expected red span: %s", out)
	}
	if !strings.Contains(out, "Signature:") || !strings.Contains(out, "os.chdir") {
		t.Fatalf("lost text: %s", out)
	}
	if !strings.Contains(out, "os.chdir(path)") {
		t.Fatalf("body missing: %s", out)
	}
	// XSS: escape
	evil := ANSIToHTML(`<script>alert(1)</script>`)
	if strings.Contains(evil, "<script>") {
		t.Fatalf("not escaped: %s", evil)
	}
	if !strings.Contains(evil, "&lt;script&gt;") {
		t.Fatalf("expected escaped: %s", evil)
	}
}

func TestANSIToHTML_c1CSIMatchesESC(t *testing.T) {
	esc := ANSIToHTML("\x1b[31mred\x1b[0m")
	c1 := ANSIToHTML("\x9b31mred\x9b0m")
	if esc != c1 {
		t.Fatalf("C1 CSI diverged\nesc: %s\nc1:  %s", esc, c1)
	}
	if !strings.Contains(c1, `class="ansi-fg-red"`) || !strings.Contains(c1, "red") {
		t.Fatalf("C1 SGR not applied: %s", c1)
	}
}

func TestANSIToHTML_truncatedCSIKeepsPrefix(t *testing.T) {
	for _, in := range []string{"hello\x1b[31", "hello\x9b31", "hello\x9b"} {
		out := ANSIToHTML(in)
		if !strings.Contains(out, "hello") {
			t.Fatalf("lost prefix for %q: %s", in, out)
		}
		if strings.Contains(out, "\x1b") || strings.Contains(out, "\x9b") {
			t.Fatalf("raw CSI leaked for %q: %s", in, out)
		}
	}
}
