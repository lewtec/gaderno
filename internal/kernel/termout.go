package kernel

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Visible-buffer bounds for TermFilter. Real notebook streams stay well under
// these; hostile/buggy kernels can otherwise force multi-GiB allocations with
// a few bytes of CSI (cursor-down 1e9, CUP huge column, etc.).
const (
	termMaxRows     = 4096
	termMaxCols     = 8192
	termMaxCSIParam = 1_000_000 // absurd for terminals; still hard-capped
)

// TermFilter turns terminal-oriented stream output into plain text suitable for
// notebook cells: strips ANSI/OSC/private modes, and applies a small set of
// cursor/line controls so progress spinners (CR, erase-line, cursor-up) collapse
// instead of stacking as garbage frames.
type TermFilter struct {
	lines [][]rune
	row   int
	col   int
}

// Write consumes raw terminal bytes/text and updates the visible buffer.
func (t *TermFilter) Write(s string) {
	if t.lines == nil {
		t.lines = [][]rune{nil}
	}
	i := 0
	for i < len(s) {
		// ESC-introduced sequences
		if s[i] == 0x1b {
			i = t.consumeESC(s, i)
			continue
		}
		// C1 CSI (0x9b)
		if s[i] == 0x9b {
			i = t.consumeCSI(s, i+1)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		i += size
		switch r {
		case '\n':
			t.newline()
		case '\r':
			t.col = 0
		case '\b':
			if t.col > 0 {
				t.col--
			}
		case '\t':
			// advance to next 8-col stop
			next := ((t.col / 8) + 1) * 8
			for t.col < next {
				t.put(' ')
			}
		case '\a': // bell
			// ignore
		default:
			if r < 0x20 || r == 0x7f {
				// other C0 controls
				continue
			}
			t.put(r)
		}
	}
}

// String returns the filtered plain text.
func (t *TermFilter) String() string {
	if len(t.lines) == 0 {
		return ""
	}
	// Drop trailing empty line only if cursor sits on a blank final row
	// that was opened by a trailing newline — keep intentional content.
	var b strings.Builder
	for i, line := range t.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(line))
	}
	return b.String()
}

// FilterTerminal is a one-shot convenience: raw terminal text → plain text.
func FilterTerminal(s string) string {
	var f TermFilter
	f.Write(s)
	return f.String()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	return min(v, hi)
}

func (t *TermFilter) clampCursor() {
	t.row = clampInt(t.row, 0, termMaxRows-1)
	t.col = clampInt(t.col, 0, termMaxCols-1)
}

func (t *TermFilter) ensureRow() {
	t.clampCursor()
	for len(t.lines) <= t.row {
		t.lines = append(t.lines, nil)
	}
}

func (t *TermFilter) put(r rune) {
	t.clampCursor()
	// Line already full: drop further glyphs rather than growing past the cap.
	if t.col >= termMaxCols {
		return
	}
	t.ensureRow()
	line := t.lines[t.row]
	if t.col < len(line) {
		line[t.col] = r
	} else {
		// Pad only up to col (already clamped); never beyond termMaxCols.
		for len(line) < t.col && len(line) < termMaxCols {
			line = append(line, ' ')
		}
		if len(line) >= termMaxCols {
			t.lines[t.row] = line[:termMaxCols]
			return
		}
		line = append(line, r)
	}
	if len(line) > termMaxCols {
		line = line[:termMaxCols]
	}
	t.lines[t.row] = line
	t.col++
	if t.col > termMaxCols {
		t.col = termMaxCols
	}
}

func (t *TermFilter) newline() {
	t.col = 0
	if t.row < termMaxRows-1 {
		t.row++
		t.ensureRow()
		return
	}
	// At bottom of the window: scroll oldest line off so long honest streams
	// keep the tail instead of stalling on the last row forever.
	if len(t.lines) > 0 {
		t.lines = t.lines[1:]
	}
	t.lines = append(t.lines, nil)
	t.row = len(t.lines) - 1
	t.clampCursor()
}

func (t *TermFilter) consumeESC(s string, i int) int {
	// i points at ESC
	if i+1 >= len(s) {
		return len(s)
	}
	next := s[i+1]
	switch next {
	case '[': // CSI
		return t.consumeCSI(s, i+2)
	case ']': // OSC … BEL or ST
		return consumeStringTerm(s, i+2)
	case 'P', 'X', '^', '_': // DCS/SOS/PM/APC … ST
		return consumeStringTerm(s, i+2)
	case 'c', '7', '8', 'D', 'E', 'H', 'M', 'Z', '>', '=', 'N', 'O':
		// single-byte ESC commands / shifts
		return i + 2
	case '(', ')', '*', '+': // charset designation ESC ( B etc.
		if i+2 < len(s) {
			return i + 3
		}
		return len(s)
	case '#': // ESC # n
		if i+2 < len(s) {
			return i + 3
		}
		return len(s)
	default:
		// unknown 2-byte ESC — drop ESC only and continue (safer: skip next)
		return i + 2
	}
}

// consumeCSI starts after the CSI introducer (ESC[ or 0x9b).
func (t *TermFilter) consumeCSI(s string, i int) int {
	// CSI params: digits, ;, ?, >, =, spaces, intermediate bytes 0x20-0x2F
	// final byte: 0x40-0x7E
	start := i
	for i < len(s) {
		c := s[i]
		if c >= 0x40 && c <= 0x7e {
			// final
			params := s[start:i]
			final := c
			i++
			t.applyCSI(params, final)
			return i
		}
		// allow parameter and intermediate bytes
		if c >= 0x20 && c <= 0x3f {
			i++
			continue
		}
		// malformed — stop
		return i + 1
	}
	return len(s)
}

func (t *TermFilter) applyCSI(params string, final byte) {
	// strip private markers (?, >, =) from front for parsing n
	p := params
	for len(p) > 0 && (p[0] == '?' || p[0] == '>' || p[0] == '=' || p[0] == '!') {
		p = p[1:]
	}
	switch final {
	case 'm': // SGR colors/styles
		return
	case 'K': // erase in line
		n := csiNum(p, 0)
		t.ensureRow()
		line := t.lines[t.row]
		switch n {
		case 0: // erase to end of line
			if t.col < len(line) {
				t.lines[t.row] = line[:t.col]
			}
		case 1: // erase to start
			for i := 0; i < t.col && i < len(line); i++ {
				line[i] = ' '
			}
			t.lines[t.row] = line
		case 2: // erase entire line
			t.lines[t.row] = nil
			// col stays; typical tools reset with CR after/before
		}
	case 'J': // erase in display — treat as clear-from-cursor for simplicity
		n := csiNum(p, 0)
		t.ensureRow()
		switch n {
		case 0: // erase below
			if t.col < len(t.lines[t.row]) {
				t.lines[t.row] = t.lines[t.row][:t.col]
			}
			if t.row+1 < len(t.lines) {
				t.lines = t.lines[:t.row+1]
			}
		case 1: // erase above
			for r := 0; r < t.row; r++ {
				t.lines[r] = nil
			}
			line := t.lines[t.row]
			for i := 0; i < t.col && i < len(line); i++ {
				line[i] = ' '
			}
			t.lines[t.row] = line
		case 2, 3: // clear screen
			t.lines = [][]rune{nil}
			t.row = 0
			t.col = 0
		}
	case 'A': // cursor up
		n := csiNum(p, 1)
		t.row -= n
		t.clampCursor()
	case 'B': // cursor down
		n := csiNum(p, 1)
		t.row += n
		t.ensureRow()
	case 'C': // cursor forward
		n := csiNum(p, 1)
		t.col += n
		t.clampCursor()
	case 'D': // cursor back
		n := csiNum(p, 1)
		t.col -= n
		t.clampCursor()
	case 'G': // cursor horizontal absolute (1-based)
		n := csiNum(p, 1)
		t.col = n - 1
		t.clampCursor()
	case 'H', 'f': // cursor position row;col (1-based)
		row, col := 1, 1
		if parts := strings.Split(p, ";"); len(parts) >= 1 {
			if parts[0] != "" {
				row = csiNum(parts[0], 1)
			}
			if len(parts) >= 2 && parts[1] != "" {
				col = csiNum(parts[1], 1)
			}
		}
		t.row = row - 1
		t.col = col - 1
		t.ensureRow()
	case 's', 'u': // save/restore cursor — ignore
	case 'n', 'c', 'p': // device status / DA / DECRQM replies — ignore
	default:
		// ignore other CSI (modes h/l, scroll, etc.)
	}
}

func csiNum(p string, def int) int {
	p = strings.TrimSpace(p)
	if p == "" {
		return def
	}
	// take first number only
	if i := strings.IndexByte(p, ';'); i >= 0 {
		p = p[:i]
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return def
	}
	if n < 0 {
		return def
	}
	return min(n, termMaxCSIParam)
}

// consumeStringTerm skips an OSC/DCS/… payload until BEL or ST (ESC \).
// OSC and ST-terminated sequences share the same terminators (order does not matter).
func consumeStringTerm(s string, i int) int {
	for i < len(s) {
		if s[i] == 0x07 { // BEL
			return i + 1
		}
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' { // ST
			return i + 2
		}
		i++
	}
	return len(s)
}


