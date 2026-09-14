package kernel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Message is a Jupyter wire message (protocol 5.x).
type Message struct {
	Header       Header
	ParentHeader Header
	Metadata     map[string]any
	Content      map[string]any
	Buffers      [][]byte
}

// Header is a Jupyter message header.
type Header struct {
	MsgID    string `json:"msg_id"`
	Session  string `json:"session"`
	Username string `json:"username"`
	Date     string `json:"date"`
	MsgType  string `json:"msg_type"`
	Version  string `json:"version"`
}

// NewHeader builds a protocol 5.3 header.
func NewHeader(session, msgType string) Header {
	return Header{
		MsgID:    uuid.NewString(),
		Session:  session,
		Username: "gaderno",
		Date:     time.Now().UTC().Format(time.RFC3339Nano),
		MsgType:  msgType,
		Version:  "5.3",
	}
}

// EncodeWire builds multipart frames: [IDS|MSG, hmac, header, parent, meta, content, buffers...]
// identities are optional leading frames before the delimiter (not included here — caller prepends).
func EncodeWire(key []byte, msg Message) ([][]byte, error) {
	header, err := json.Marshal(msg.Header)
	if err != nil {
		return nil, err
	}
	var parent []byte
	if msg.ParentHeader.MsgID == "" && msg.ParentHeader.MsgType == "" {
		parent = []byte("{}")
	} else {
		parent, err = json.Marshal(msg.ParentHeader)
		if err != nil {
			return nil, err
		}
	}
	meta := msg.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaRaw, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	content := msg.Content
	if content == nil {
		content = map[string]any{}
	}
	contentRaw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	sig := sign(key, header, parent, metaRaw, contentRaw)
	frames := [][]byte{
		[]byte("<IDS|MSG>"),
		[]byte(sig),
		header,
		parent,
		metaRaw,
		contentRaw,
	}
	frames = append(frames, msg.Buffers...)
	return frames, nil
}

// DecodeWire parses frames starting at the delimiter (or including identities before it).
func DecodeWire(key []byte, frames [][]byte) (Message, error) {
	i := 0
	for i < len(frames) && string(frames[i]) != "<IDS|MSG>" {
		i++
	}
	if i >= len(frames) {
		return Message{}, fmt.Errorf("missing <IDS|MSG> delimiter")
	}
	rest := frames[i+1:]
	if len(rest) < 5 {
		return Message{}, fmt.Errorf("truncated message: %d frames after delimiter", len(rest))
	}
	sig, header, parent, meta, content := rest[0], rest[1], rest[2], rest[3], rest[4]
	expect := sign(key, header, parent, meta, content)
	// hmac.Equal → subtle.ConstantTimeCompare panics when lengths differ.
	// Malformed or hostile signature frames must not kill the ZMQ readLoop.
	if len(key) > 0 && !hmacEqualSafe(sig, []byte(expect)) {
		return Message{}, fmt.Errorf("invalid HMAC signature")
	}
	var msg Message
	if err := json.Unmarshal(header, &msg.Header); err != nil {
		return Message{}, fmt.Errorf("header: %w", err)
	}
	if err := json.Unmarshal(parent, &msg.ParentHeader); err != nil {
		return Message{}, fmt.Errorf("parent: %w", err)
	}
	if err := json.Unmarshal(meta, &msg.Metadata); err != nil {
		return Message{}, fmt.Errorf("metadata: %w", err)
	}
	if err := json.Unmarshal(content, &msg.Content); err != nil {
		return Message{}, fmt.Errorf("content: %w", err)
	}
	if len(rest) > 5 {
		msg.Buffers = rest[5:]
	}
	return msg, nil
}

func sign(key, header, parent, meta, content []byte) string {
	if len(key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(header)
	mac.Write(parent)
	mac.Write(meta)
	mac.Write(content)
	return hex.EncodeToString(mac.Sum(nil))
}

// hmacEqualSafe is length-tolerant: unequal lengths are a failed verify, not a panic.
func hmacEqualSafe(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal(a, b)
}
