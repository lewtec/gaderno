package kernel

import (
	"context"
	"errors"
)

// Sentinel errors for kernel I/O and lifecycle (errors.Is).
// Shared across inspect/complete/execute/manager/zmq/spawn/catalog.
var (
	ErrNoConnection       = errors.New("no connection")
	ErrInspectTimeout     = errors.New("inspect timeout")
	ErrCompleteTimeout    = errors.New("complete timeout")
	ErrExecuteTimeout     = errors.New("execute timeout")
	ErrDialTimeout        = errors.New("dial timeout")
	ErrEmptyArgv          = errors.New("empty argv")
	ErrShellClosed        = errors.New("shell closed")
	ErrIOPubClosed        = errors.New("iopub closed")
	ErrConnectionClosed   = errors.New("connection closed")
	ErrKernelExitedEarly  = errors.New("kernel process exited early")
	ErrKernelspecNotFound = errors.New("kernelspec not found")
)

// InspectResult is the Jupyter inspect_reply payload we surface to the UI.
type InspectResult struct {
	Status string `json:"status"`
	Found  bool   `json:"found"`
	// Text is plain text (ANSI stripped) from text/plain.
	Text string `json:"text"`
	// HTML is safe colored markup (ANSI → classed spans, content escaped).
	HTML string `json:"html"`
	// DetailLevel echoes the request (0 ≈ signature, 1 ≈ full docs).
	DetailLevel int `json:"detail_level"`
}

// MaxInspectBytes caps inspect text/plain (or fallback text/html) before we
// build ANSI HTML / plain text for the client. Mirrors the large-output policy
// (display mimes use MaxDisplayBytes); tooltips do not need multi-MiB docs.
const MaxInspectBytes = 256 << 10 // 256 KiB

// Inspect asks the kernel for docs/signature at cursorPos.
// detailLevel: 0 abbreviated, 1 full (Jupyter protocol).
// Best-effort: returns Found=false when shell is busy with execute.
func (m *Manager) Inspect(ctx context.Context, code string, cursorPos, detailLevel int) (InspectResult, error) {
	if m.Conn == nil {
		return InspectResult{}, ErrNoConnection
	}
	// Reuse complete code window so huge cells do not inflate shell traffic.
	code, cursorPos = clampCompleteCode(code, cursorPos)
	if detailLevel < 0 {
		detailLevel = 0
	}
	if detailLevel > 1 {
		detailLevel = 1
	}

	req := Message{
		Header: NewHeader(m.Session, "inspect_request"),
		Content: map[string]any{
			"code":         code,
			"cursor_pos":   cursorPos,
			"detail_level": detailLevel,
		},
	}
	msg, err := m.shellRequest(ctx, req, "inspect_reply", ErrInspectTimeout)
	if errors.Is(err, errShellBusy) {
		return InspectResult{Status: "busy", Found: false, DetailLevel: detailLevel}, nil
	}
	if err != nil {
		return InspectResult{}, err
	}
	return parseInspectReply(msg.Content, detailLevel), nil
}

func parseInspectReply(content map[string]any, detailLevel int) InspectResult {
	res := InspectResult{
		Status:      "ok",
		Found:       false,
		DetailLevel: detailLevel,
	}
	if content == nil {
		return res
	}
	if s, ok := content["status"].(string); ok && s != "" {
		res.Status = s
	}
	switch f := content["found"].(type) {
	case bool:
		res.Found = f
	case float64:
		res.Found = f != 0
	}
	var rawPlain string
	if data, ok := content["data"].(map[string]any); ok {
		// Prefer text/plain (often ANSI-colored by ipykernel).
		if tp, ok := data["text/plain"]; ok {
			rawPlain = multilineContent(tp)
		} else if th, ok := data["text/html"]; ok {
			// No safe HTML sanitizer for arbitrary kernel HTML — plain-strip only.
			rawPlain = multilineContent(th)
		}
	}
	if rawPlain != "" {
		if len(rawPlain) > MaxInspectBytes {
			rawPlain = truncateUTF8(rawPlain, MaxInspectBytes)
			rawPlain += "\n[gaderno: truncated inspect output]"
		}
		// Colored tooltip markup (escaped text + classed spans).
		res.HTML = ANSIToHTML(rawPlain)
		// Plain fallback for clients / signature-line extraction.
		res.Text = FilterTerminal(rawPlain)
	}
	if res.Text != "" {
		// Some kernels omit found=true but still send a body.
		res.Found = true
	}
	return res
}
