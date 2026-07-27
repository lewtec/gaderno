package kernel

import (
	"errors"
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
	if !errors.Is(err, ErrInvalidHMAC) {
		t.Fatalf("DecodeWire bad key: got %v, want ErrInvalidHMAC", err)
	}
}

func TestWireMissingDelimiter(t *testing.T) {
	_, err := DecodeWire([]byte("k"), [][]byte{[]byte("no-delimiter"), []byte("x")})
	if !errors.Is(err, ErrMissingDelimiter) {
		t.Fatalf("DecodeWire missing delimiter: got %v, want ErrMissingDelimiter", err)
	}
}

func TestWireTruncatedMessage(t *testing.T) {
	// Delimiter present but fewer than 5 frames after it.
	frames := [][]byte{[]byte("<IDS|MSG>"), []byte("sig"), []byte("{}")}
	_, err := DecodeWire([]byte("k"), frames)
	if !errors.Is(err, ErrTruncatedMessage) {
		t.Fatalf("DecodeWire truncated: got %v, want ErrTruncatedMessage", err)
	}
	if err == nil || err.Error() == "" {
		t.Fatal("expected frame count in message")
	}
	if want := "truncated message: 2 frames after delimiter"; err.Error() != want {
		t.Fatalf("message: got %q, want %q", err.Error(), want)
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
			if !errors.Is(err, ErrInvalidHMAC) {
				t.Fatalf("DecodeWire sig len=%d: got %v, want ErrInvalidHMAC", len(bad), err)
			}
		}()
	}
}
