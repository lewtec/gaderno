package app

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lucasew/gaderno/internal/document"
	"github.com/lucasew/gaderno/internal/session"
	"github.com/lucasew/gaderno/internal/store"
)

func TestMaxWSMessageBytesMatchesDisplayCap(t *testing.T) {
	// Keep inbound WS bound aligned with kernel mime/stream soft caps.
	const want = 12 << 20
	if MaxWSMessageBytes != want {
		t.Fatalf("MaxWSMessageBytes=%d want %d", MaxWSMessageBytes, want)
	}
}

func TestWSReadLimitRejectsOversizeBinary(t *testing.T) {
	dir := t.TempDir()
	st := store.New(dir)
	nb := document.NewEmpty()
	if err := st.Save(t.Context(), "n.ipynb", nb); err != nil {
		t.Fatal(err)
	}
	reg := session.NewRegistry(st, dir)
	defer reg.CloseAll(t.Context())

	mux := http.NewServeMux()
	registerWS(mux, reg, slog.Default())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/notebooks/n.ipynb"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Drain server hello so the peer is live.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("hello: %v", err)
	}

	// One byte over the assembled-message cap must trip the server read limit.
	// Use a smaller temporary limit via direct write of MaxWSMessageBytes+1.
	// (Writing 12MiB+ in CI is fine under 4GiB but keep timeout generous.)
	payload := make([]byte, MaxWSMessageBytes+1)
	if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		// Client may see a close while writing if the peer already dropped.
		t.Logf("write after oversize (may fail): %v", err)
	}

	// Server should close the connection; subsequent read fails.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected read error after oversize message")
	}
}
