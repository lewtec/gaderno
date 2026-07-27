package kernel

import (
	"errors"
	"testing"
)

func TestCompleteBusyWhenShellLocked(t *testing.T) {
	m := &Manager{Conn: &Conn{}, Session: "s"}
	m.shellMu.Lock()
	t.Cleanup(m.shellMu.Unlock)

	res, err := m.Complete(t.Context(), "os.", 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Status != "busy" {
		t.Fatalf("status %q want busy", res.Status)
	}
	if res.CursorStart != 3 || res.CursorEnd != 3 {
		t.Fatalf("cursors %d %d", res.CursorStart, res.CursorEnd)
	}
}

func TestInspectBusyWhenShellLocked(t *testing.T) {
	m := &Manager{Conn: &Conn{}, Session: "s"}
	m.shellMu.Lock()
	t.Cleanup(m.shellMu.Unlock)

	res, err := m.Inspect(t.Context(), "print", 0, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Status != "busy" || res.Found || res.DetailLevel != 1 {
		t.Fatalf("%#v", res)
	}
}

func TestShellRequestNilConnPathStillNoConnection(t *testing.T) {
	// Complete/Inspect gate on Conn before shellRequest; keep that contract.
	m := &Manager{}
	_, err := m.Complete(t.Context(), "x", 0)
	if !errors.Is(err, ErrNoConnection) {
		t.Fatalf("complete: %v", err)
	}
}
