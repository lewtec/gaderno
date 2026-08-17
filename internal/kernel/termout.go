package kernel

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/vt"
)

// TermFilter turns terminal-oriented stream output into plain text for
// notebook cells. It is a real VT at a fixed size (default 80×24, matching
// COLUMNS/LINES) plus scrollback — TUIs address the viewport with CSI nA/nB,
// not an unbounded tape of rows.
type TermFilter struct {
	em *vt.Emulator
}

func NewTermFilter(cols, rows int) *TermFilter {
	if cols < 1 {
		cols = kernelTermCols
	}
	if rows < 1 {
		rows = kernelTermRows
	}
	if cols > termMaxCols {
		cols = termMaxCols
	}
	if rows > termMaxRows {
		rows = termMaxRows
	}
	em := vt.NewEmulator(cols, rows)
	em.SetScrollbackSize(termMaxRows)
	// x/vt replies to DA/DECRQM on an unbuffered pipe. Nobody reads that
	// in a notebook, so the next query would block the Write forever.
	go io.Copy(io.Discard, em)
	return &TermFilter{em: em}
}

// Close releases the emulator input pipe. Safe on a zero TermFilter.
func (t *TermFilter) Close() {
	if t == nil || t.em == nil {
		return
	}
	_ = t.em.Close()
	t.em = nil
}

func (t *TermFilter) ensure() {
	if t.em == nil {
		*t = *NewTermFilter(kernelTermCols, kernelTermRows)
	}
}

// Write consumes raw terminal bytes/text and updates the visible buffer.
func (t *TermFilter) Write(s string) {
	t.ensure()
	// Raw VT treats LF as "same column, next row". Notebook streams are
	// Unix text (`print`, slog): LF means newline. Map lone LF → CRLF
	// (ONLCR). Recorded PTYs already send CR LF; those stay put.
	_, _ = t.em.WriteString(onlcr(s))
}

func onlcr(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && (i == 0 || s[i-1] != '\r') {
			b.WriteByte('\r')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// String returns scrollback plus the current viewport as plain text.
func (t *TermFilter) String() string {
	if t.em == nil {
		return ""
	}
	return dumpVT(t.em)
}

// FilterTerminal is a one-shot convenience for inspect/traceback/evalue:
// apply VT (CR, SGR, OSC) on a wide screen so a long line is not wrapped.
// Live execute streams use NewTermFilter(80, 24) instead.
func FilterTerminal(s string) string {
	f := NewTermFilter(256, 256)
	defer f.Close()
	f.Write(s)
	return f.String()
}

const (
	termMaxRows = 4096
	termMaxCols = 8192
)

func dumpVT(em *vt.Emulator) string {
	var b strings.Builder
	if sb := em.Scrollback(); sb != nil {
		for _, line := range sb.Lines() {
			b.WriteString(strings.TrimRight(line.String(), " "))
			b.WriteByte('\n')
		}
	}
	for y := 0; y < em.Height(); y++ {
		b.WriteString(screenRow(em, y))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func screenRow(em *vt.Emulator, y int) string {
	var row strings.Builder
	for x := 0; x < em.Width(); {
		c := em.CellAt(x, y)
		if c == nil || c.IsZero() {
			row.WriteByte(' ')
			x++
			continue
		}
		s := c.String()
		if s == "" {
			row.WriteByte(' ')
			x++
			continue
		}
		row.WriteString(s)
		w := c.Width
		if w < 1 {
			w = utf8.RuneCountInString(s)
		}
		if w < 1 {
			w = 1
		}
		x += w
	}
	return strings.TrimRight(row.String(), " ")
}
