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
