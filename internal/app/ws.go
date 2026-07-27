package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lucasew/gaderno/internal/jsonutil"
	"github.com/lucasew/gaderno/internal/kernel"
	"github.com/lucasew/gaderno/internal/session"
)

// MaxWSMessageBytes caps a single inbound WebSocket frame (text control or
// binary CRDT sync). Matches the kernel display/stream soft cap so one peer
// cannot OOM the process with multi-GB updates (SPEC: CRDT spam size limits).
const MaxWSMessageBytes = 12 << 20 // 12 MiB

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64,
	WriteBufferSize: 1024 * 64,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsControl struct {
	Type        string `json:"type"`
	CellID      string `json:"cell_id,omitempty"`
	Text        string `json:"text,omitempty"`
	Source      string `json:"source,omitempty"`
	Name        string `json:"name,omitempty"`
	Update      string `json:"update,omitempty"` // base64 awareness payload
	Index       *int   `json:"index,omitempty"`
	Code        string `json:"code,omitempty"`
	CursorPos   *int   `json:"cursor_pos,omitempty"`
	ReqID       string `json:"req_id,omitempty"`
	DetailLevel *int   `json:"detail_level,omitempty"`
}

func registerWS(mux *http.ServeMux, reg *session.Registry, logger *slog.Logger) {
	mux.HandleFunc("GET /ws/notebooks/{path...}", func(w http.ResponseWriter, r *http.Request) {
		reqCtx := r.Context()
		path := r.PathValue("path")
		hub, err := reg.GetOrOpen(reqCtx, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("ws upgrade", "err", err)
			return
		}
		// Bound assembled message size before any CRDT/control handling.
		conn.SetReadLimit(MaxWSMessageBytes)
		clientID := uuid.NewString()
		client := hub.AddClient(clientID)
		defer hub.RemoveClient(clientID)
		defer conn.Close()

		// Single writer goroutine — gorilla/websocket is not concurrent-safe.
		done := make(chan struct{})
		go func() {
			defer close(done)
			for out := range client.Out {
				if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
					return
				}
				mt := websocket.TextMessage
				if out.Binary {
					mt = websocket.BinaryMessage
				}
				if err := conn.WriteMessage(mt, out.Data); err != nil {
					return
				}
			}
		}()

		// hello only until client acks — do not push CRDT state or accept
		// client updates before the session fence passes (prevents a tab with
		// a previous Y.Doc from poisoning a recreated hub on reconnect).
		hello, err := json.Marshal(map[string]string{
			"type":       "hello",
			"session_id": hub.SessionID,
			"client_id":  clientID,
		})
		if err != nil {
			logger.Error("ws hello marshal", "err", err)
			return
		}
		client.Out <- session.Outbound{Data: hello}

		for {
			if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
				break
			}
			mt, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			switch mt {
			case websocket.BinaryMessage:
				reply, err := hub.HandleSyncMessage(clientID, data)
				if err != nil {
					logger.Debug("sync apply", "err", err)
					continue
				}
				if reply != nil {
					select {
					case client.Out <- session.Outbound{Binary: true, Data: reply}:
					default:
					}
				}
			case websocket.TextMessage:
				var ctrl wsControl
				if err := json.Unmarshal(data, &ctrl); err != nil {
					continue
				}
				if ctrl.Type == "hello.ack" {
					var ack struct {
						SessionID string `json:"session_id"`
					}
					if err := json.Unmarshal(data, &ack); err != nil {
						continue
					}
					if ack.SessionID != "" && ack.SessionID != hub.SessionID {
						sendErr(client, "session_id mismatch")
						continue
					}
					if !hub.MarkClientReady(clientID) {
						continue
					}
					hub.SendKernelStatus(client)
					select {
					case client.Out <- session.Outbound{Binary: true, Data: hub.EncodeSyncStep1()}:
					default:
					}
					continue
				}
				if !hub.ClientReady(clientID) {
					// Drop awareness/control until session is confirmed.
					continue
				}
				// Awareness: pass through raw JSON so "update" is preserved.
				if ctrl.Type == "awareness" {
					hub.BroadcastJSON(data, clientID)
					continue
				}
				handleControl(reqCtx, hub, client, clientID, ctrl, logger)
			}
		}
		<-done
	})
}

func handleControl(ctx context.Context, hub *session.Hub, client *session.Client, clientID string, ctrl wsControl, logger *slog.Logger) {
	// Detach from the HTTP request cancel so long work survives brief WS flaps.
	ctx = context.WithoutCancel(ctx)
	switch ctrl.Type {
	case "ping":
		select {
		case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]string{"type": "pong"})}:
		default:
		}
	case "chat.send":
		hub.BroadcastJSON(jsonutil.Bytes(map[string]string{
			"type": "chat.message",
			"text": ctrl.Text,
			"from": client.ID[:8],
		}), "")
	case "cell.set_source":
		// Legacy full-cell replace (still used as Run flush safety).
		if ctrl.CellID == "" {
			sendErr(client, "cell_id required")
			return
		}
		if err := hub.SetCellSource(ctrl.CellID, ctrl.Source, clientID); err != nil {
			sendErr(client, err.Error())
			return
		}
		select {
		case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{
			"type":    "cell.source_ack",
			"cell_id": ctrl.CellID,
		})}:
		default:
		}
	case "cell.insert":
		idx := 0
		if ctrl.Index != nil {
			idx = *ctrl.Index
		} else {
			// append by default
			idx = len(hub.Doc.SnapshotCells())
		}
		ct := ctrl.Text // "code" | "markdown"
		if ct == "" {
			ct = "code"
		}
		id, err := hub.InsertCell(idx, ct)
		if err != nil {
			sendErr(client, err.Error())
			return
		}
		// structure broadcast already sent; include focus hint to originator
		select {
		case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{"type": "cell.inserted", "cell_id": id, "index": idx})}:
		default:
		}
	case "cell.delete":
		if ctrl.CellID == "" {
			sendErr(client, "cell_id required")
			return
		}
		if err := hub.DeleteCell(ctrl.CellID); err != nil {
			sendErr(client, err.Error())
		}
	case "cell.set_type":
		if ctrl.CellID == "" {
			sendErr(client, "cell_id required")
			return
		}
		ct := ctrl.Text
		if ct == "" {
			ct = ctrl.Name
		}
		if err := hub.SetCellType(ctrl.CellID, ct); err != nil {
			sendErr(client, err.Error())
		}
	case "cell.move":
		if ctrl.CellID == "" || ctrl.Index == nil {
			sendErr(client, "cell_id and index required")
			return
		}
		if err := hub.MoveCell(ctrl.CellID, *ctrl.Index); err != nil {
			sendErr(client, err.Error())
		}
	case "kernel.bind":
		name := ctrl.Name
		if name == "" {
			name = ctrl.Text
		}
		if err := hub.BindKernel(name); err != nil {
			sendErr(client, err.Error())
		}
	case "exec.run":
		go func() {
			// Prefer live CRDT text; client source is a flush backup.
			if ctrl.CellID != "" && ctrl.Source != "" {
				if err := hub.SetCellSource(ctrl.CellID, ctrl.Source, clientID); err != nil {
					// continue with CRDT source
				}
			}
			ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			if err := hub.EnsureKernel(ctx, ""); err != nil {
				if errors.Is(err, session.ErrNoKernelSelected) {
					select {
					case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{"type": "kernel.needs_pick"})}:
					default:
					}
				}
				sendErr(client, err.Error())
				return
			}
			res, err := hub.ExecuteCell(ctx, ctrl.CellID,
				func(ch kernel.StreamChunk) {
					hub.BroadcastJSON(jsonutil.Bytes(map[string]any{
						"type":    "exec.stream",
						"cell_id": ctrl.CellID,
						"name":    ch.Name,
						"text":    ch.Text,
					}), "")
				},
				func(dd kernel.DisplayData) {
					// Full mime bundle — client chooses renderers.
					hub.BroadcastJSON(jsonutil.Bytes(map[string]any{
						"type":        "exec.display",
						"cell_id":     ctrl.CellID,
						"output_type": dd.OutputType,
						"data":        dd.Data,
						"metadata":    dd.Metadata,
						"transient":   dd.Transient,
					}), "")
				},
			)
			if err != nil {
				sendErr(client, err.Error())
				return
			}
			hub.BroadcastJSON(jsonutil.Bytes(map[string]any{
				"type":            "exec.result",
				"cell_id":         ctrl.CellID,
				"status":          res.Status,
				"stdout":          res.Stdout,
				"stderr":          res.Stderr,
				"ename":           res.Ename,
				"evalue":          res.Evalue,
				"traceback":       res.Traceback,
				"execution_count": res.ExecutionCount,
			}), "")
		}()
	case "complete.request":
		// Async; reply only to requesting client (not broadcast).
		go func() {
			code := ctrl.Code
			if code == "" {
				code = ctrl.Source
			}
			pos := 0
			if ctrl.CursorPos != nil {
				pos = *ctrl.CursorPos
			} else if len(code) > 0 {
				pos = len(code)
			}
			reqID := ctrl.ReqID
			ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			res, err := hub.Complete(ctx, code, pos)
			if err != nil {
				select {
				case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{
					"type":         "complete.reply",
					"req_id":       reqID,
					"status":       "error",
					"matches":      []string{},
					"cursor_start": pos,
					"cursor_end":   pos,
					"text":         err.Error(),
				})}:
				default:
				}
				return
			}
			select {
			case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{
				"type":         "complete.reply",
				"req_id":       reqID,
				"status":       res.Status,
				"matches":      res.Matches,
				"cursor_start": res.CursorStart,
				"cursor_end":   res.CursorEnd,
			})}:
			default:
			}
		}()
	case "inspect.request":
		// Hover / signature help — reply only to originator.
		go func() {
			code := ctrl.Code
			if code == "" {
				code = ctrl.Source
			}
			pos := 0
			if ctrl.CursorPos != nil {
				pos = *ctrl.CursorPos
			} else if len(code) > 0 {
				pos = len(code)
			}
			detail := 0
			if ctrl.DetailLevel != nil {
				detail = *ctrl.DetailLevel
			}
			reqID := ctrl.ReqID
			ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			res, err := hub.Inspect(ctx, code, pos, detail)
			if err != nil {
				select {
				case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{
					"type":         "inspect.reply",
					"req_id":       reqID,
					"status":       "error",
					"found":        false,
					"text":         err.Error(),
					"detail_level": detail,
				})}:
				default:
				}
				return
			}
			select {
			case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]any{
				"type":         "inspect.reply",
				"req_id":       reqID,
				"status":       res.Status,
				"found":        res.Found,
				"text":         res.Text,
				"html":         res.HTML,
				"detail_level": res.DetailLevel,
			})}:
			default:
			}
		}()
	}
}

func sendErr(client *session.Client, msg string) {
	select {
	case client.Out <- session.Outbound{Data: jsonutil.Bytes(map[string]string{"type": "error", "text": msg})}:
	default:
	}
}
