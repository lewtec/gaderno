package kernel

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"
)

// CompleteResult is the Jupyter complete_reply payload we care about.
type CompleteResult struct {
	Matches     []string `json:"matches"`
	CursorStart int      `json:"cursor_start"`
	CursorEnd   int      `json:"cursor_end"`
	Status      string   `json:"status"`
}

// MaxCompleteMatches caps autocomplete suggestions forwarded to clients.
// Unbounded complete_reply lists (huge namespaces / hostile kernels) can OOM
// the process and flood the WebSocket JSON fan-out.
const MaxCompleteMatches = 500

// MaxCompleteMatchBytes caps a single match string (identifier / path snippet).
const MaxCompleteMatchBytes = 512

// MaxCompleteCodeBytes caps code sent on complete_request (same order as
// typical cell sources; full multi-MiB dumps are not useful for autocomplete).
const MaxCompleteCodeBytes = 1 << 20 // 1 MiB

// Complete asks the kernel for completions at cursorPos (byte/UTF-8 offset in code).
// Returns empty matches (not error) when the shell is busy with execute.
func (m *Manager) Complete(ctx context.Context, code string, cursorPos int) (CompleteResult, error) {
	if m.Conn == nil {
		return CompleteResult{}, fmt.Errorf("no connection")
	}
	code, cursorPos = clampCompleteCode(code, cursorPos)

	// Do not block execute; autocomplete is best-effort.
	if !m.shellMu.TryLock() {
		return CompleteResult{Status: "busy", CursorStart: cursorPos, CursorEnd: cursorPos}, nil
	}
	defer m.shellMu.Unlock()

	req := Message{
		Header: NewHeader(m.Session, "complete_request"),
		Content: map[string]any{
			"code":       code,
			"cursor_pos": cursorPos,
		},
	}
	msgID := req.Header.MsgID
	if err := m.Conn.SendShell(req); err != nil {
		return CompleteResult{}, err
	}

	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	for {
		if err := ctx.Err(); err != nil {
			return CompleteResult{}, err
		}
		remain := time.Until(deadline)
		if remain <= 0 {
			return CompleteResult{}, fmt.Errorf("complete timeout")
		}
		rctx, cancel := context.WithTimeout(ctx, remain)
		msg, ch, err := m.Conn.recvEither(rctx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return CompleteResult{}, ctx.Err()
			}
			if time.Now().After(deadline) {
				return CompleteResult{}, fmt.Errorf("complete timeout")
			}
			continue
		}
		// Ignore IOPub noise; only shell complete_reply for our parent id.
		if ch != "shell" {
			continue
		}
		if msg.Header.MsgType != "complete_reply" {
			continue
		}
		if msg.ParentHeader.MsgID != msgID {
			continue
		}
		return parseCompleteReply(msg.Content, cursorPos), nil
	}
}

func clampCompleteCode(code string, cursorPos int) (string, int) {
	if cursorPos < 0 {
		cursorPos = 0
	}
	if len(code) > MaxCompleteCodeBytes {
		// Prefer keeping the cursor neighborhood when the cell is huge.
		// Center a window on the cursor so completion context is not lost.
		half := MaxCompleteCodeBytes / 2
		start := cursorPos - half
		if start < 0 {
			start = 0
		}
		end := start + MaxCompleteCodeBytes
		if end > len(code) {
			end = len(code)
			start = end - MaxCompleteCodeBytes
			if start < 0 {
				start = 0
			}
		}
		// Snap to UTF-8 rune boundaries.
		for start > 0 && !utf8.RuneStart(code[start]) {
			start--
		}
		for end < len(code) && end > start && !utf8.RuneStart(code[end]) {
			end--
		}
		cursorPos -= start
		code = code[start:end]
	}
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos > len(code) {
		cursorPos = len(code)
	}
	return code, cursorPos
}

func parseCompleteReply(content map[string]any, fallbackPos int) CompleteResult {
	res := CompleteResult{
		Status:      "ok",
		CursorStart: fallbackPos,
		CursorEnd:   fallbackPos,
		Matches:     nil,
	}
	if content == nil {
		res.Matches = []string{}
		return res
	}
	if s, ok := content["status"].(string); ok && s != "" {
		res.Status = s
	}
	if n, ok := asInt(content["cursor_start"]); ok {
		res.CursorStart = n
	}
	if n, ok := asInt(content["cursor_end"]); ok {
		res.CursorEnd = n
	}
	appendMatch := func(s string) {
		if s == "" || len(res.Matches) >= MaxCompleteMatches {
			return
		}
		if len(s) > MaxCompleteMatchBytes {
			s = truncateUTF8(s, MaxCompleteMatchBytes)
		}
		res.Matches = append(res.Matches, s)
	}
	switch m := content["matches"].(type) {
	case []any:
		for _, x := range m {
			if s, ok := x.(string); ok {
				appendMatch(s)
			}
			if len(res.Matches) >= MaxCompleteMatches {
				break
			}
		}
	case []string:
		for _, s := range m {
			appendMatch(s)
			if len(res.Matches) >= MaxCompleteMatches {
				break
			}
		}
	}
	if res.Matches == nil {
		res.Matches = []string{}
	}
	return res
}

// truncateUTF8 shortens s to at most max bytes without splitting a rune.
func truncateUTF8(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
