package kernel

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// castHeader is the first line of an asciicast v2 or v3 file.
type castHeader struct {
	Version int `json:"version"`
	Width   int `json:"width"`
	Height  int `json:"height"`
	Term    *struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	} `json:"term"`
}

func (h castHeader) cols() int {
	if h.Term != nil && h.Term.Cols > 0 {
		return h.Term.Cols
	}
	return h.Width
}

func (h castHeader) rows() int {
	if h.Term != nil && h.Term.Rows > 0 {
		return h.Term.Rows
	}
	return h.Height
}

// parseCast reads an asciicast v2/v3 stream and returns terminal size plus
// output ("o") payloads in recording order. Timing, input, markers, and
// resize events are ignored — the filter sees the same bytes the recorder did.
func parseCast(r io.Reader) (cols, rows int, chunks []string, err error) {
	sc := bufio.NewScanner(r)
	// nom / workspaced frames can be large; default 64KiB is tight.
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return 0, 0, nil, err
		}
		return 0, 0, nil, fmt.Errorf("empty cast")
	}
	var h castHeader
	if err := json.Unmarshal(sc.Bytes(), &h); err != nil {
		return 0, 0, nil, fmt.Errorf("cast header: %w", err)
	}
	if h.Version != 2 && h.Version != 3 {
		return 0, 0, nil, fmt.Errorf("cast version %d (want 2 or 3)", h.Version)
	}
	cols, rows = h.cols(), h.rows()
	if cols < 1 || rows < 1 {
		return 0, 0, nil, fmt.Errorf("cast size %dx%d", cols, rows)
	}
	line := 1
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		var ev []json.RawMessage
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			return 0, 0, nil, fmt.Errorf("cast line %d: %w", line, err)
		}
		if len(ev) < 3 {
			return 0, 0, nil, fmt.Errorf("cast line %d: want 3 fields", line)
		}
		var code string
		if err := json.Unmarshal(ev[1], &code); err != nil {
			return 0, 0, nil, fmt.Errorf("cast line %d code: %w", line, err)
		}
		if code != "o" {
			continue
		}
		var data string
		if err := json.Unmarshal(ev[2], &data); err != nil {
			return 0, 0, nil, fmt.Errorf("cast line %d data: %w", line, err)
		}
		chunks = append(chunks, data)
	}
	return cols, rows, chunks, sc.Err()
}

func parseCastFile(path string) (cols, rows int, chunks []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()
	return parseCast(f)
}

func replayCast(cols, rows int, chunks []string) string {
	f := NewTermFilter(cols, rows)
	defer f.Close()
	for _, c := range chunks {
		f.Write(c)
	}
	return f.String()
}
