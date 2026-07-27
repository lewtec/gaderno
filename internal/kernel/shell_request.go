package kernel

import (
	"context"
	"errors"
	"time"
)

// errShellBusy is returned by shellRequest when execute holds shellMu.
// Callers map it to a best-effort "busy" result (autocomplete / inspect).
var errShellBusy = errors.New("shell busy")

// shellRequest sends req on the shell channel and waits for a matching reply
// of replyType (parent msg id = request). Uses a 5s wall deadline capped by
// ctx. timeoutErr is returned when the deadline elapses without a match.
// When shellMu is held (execute in flight), returns errShellBusy without sending.
func (m *Manager) shellRequest(ctx context.Context, req Message, replyType string, timeoutErr error) (Message, error) {
	if !m.shellMu.TryLock() {
		return Message{}, errShellBusy
	}
	defer m.shellMu.Unlock()

	msgID := req.Header.MsgID
	if err := m.Conn.SendShell(req); err != nil {
		return Message{}, err
	}

	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	for {
		if err := ctx.Err(); err != nil {
			return Message{}, err
		}
		remain := time.Until(deadline)
		if remain <= 0 {
			return Message{}, timeoutErr
		}
		rctx, cancel := context.WithTimeout(ctx, remain)
		msg, ch, err := m.Conn.recvEither(rctx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return Message{}, ctx.Err()
			}
			if time.Now().After(deadline) {
				return Message{}, timeoutErr
			}
			continue
		}
		if ch != "shell" {
			continue
		}
		if msg.Header.MsgType != replyType {
			continue
		}
		if msg.ParentHeader.MsgID != msgID {
			continue
		}
		return msg, nil
	}
}
