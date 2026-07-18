package kernel

import "testing"

func TestParseInspectReply(t *testing.T) {
	res := parseInspectReply(map[string]any{
		"status": "ok",
		"found":  true,
		"data": map[string]any{
			"text/plain": "Signature: print(value, ..., sep=' ')\nDocstring:\nPrint objects",
		},
	}, 0)
	if !res.Found || res.Text == "" {
		t.Fatalf("%#v", res)
	}
	empty := parseInspectReply(nil, 1)
	if empty.Found || empty.DetailLevel != 1 {
		t.Fatalf("%#v", empty)
	}
}
