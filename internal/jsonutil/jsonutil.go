// Package jsonutil holds small JSON helpers shared across HTTP/session fan-out.
package jsonutil

import "encoding/json"

// Bytes marshals v for WebSocket/HTTP control messages.
// On failure it returns a minimal error envelope so callers can still send a frame.
func Bytes(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","text":"marshal failed"}`)
	}
	return b
}
