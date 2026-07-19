package kernel

import (
	"testing"
)

func TestWireRoundTrip(t *testing.T) {
	key := []byte("secret-key")
	session := "sess-1"
	msg := Message{
		Header:  NewHeader(session, "execute_request"),
		Content: map[string]any{"code": "1+1", "silent": false},
	}
	frames, err := EncodeWire(key, msg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeWire(key, frames)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header.MsgType != "execute_request" {
		t.Fatalf("type %q", got.Header.MsgType)
	}
	if got.Content["code"] != "1+1" {
		t.Fatalf("content %#v", got.Content)
	}
}

func TestWireBadHMAC(t *testing.T) {
	key := []byte("secret-key")
	msg := Message{Header: NewHeader("s", "kernel_info_request")}
	frames, err := EncodeWire(key, msg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeWire([]byte("other"), frames)
	if err == nil {
		t.Fatal("expected hmac error")
	}
}

func TestWireWrongLengthHMACNoPanic(t *testing.T) {
	key := []byte("secret-key")
	msg := Message{Header: NewHeader("s", "kernel_info_request")}
	frames, err := EncodeWire(key, msg)
	if err != nil {
		t.Fatal(err)
	}
	// Signature frame is frames[1] after delimiter at frames[0].
	// Truncate / inflate it: pre-fix hmac.Equal would panic on length mismatch.
	for _, bad := range [][]byte{
		[]byte("short"),
		[]byte(""),
		append(append([]byte{}, frames[1]...), 'x'),
	} {
		broken := append([][]byte{}, frames...)
		broken[1] = bad
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("DecodeWire panicked on sig len=%d: %v", len(bad), r)
				}
			}()
			_, err := DecodeWire(key, broken)
			if err == nil {
				t.Fatalf("expected hmac error for sig len=%d", len(bad))
			}
		}()
	}
}
