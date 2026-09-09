---
name: gaderno
description: >
  Talk to a running gaderno notebook server over HTTP (curl). Use when the
  user wants to create, edit, or run notebooks on gaderno, says /gaderno,
  mentions gaderno cells or kernels, or asks an agent to work in a live
  notebook instead of editing .ipynb files by hand.
---

# gaderno

The live document is on the server. Mutate it with HTTP. Do not open the WebSocket. Do not rewrite `.ipynb` files on disk.

## Connect

1. Base URL: the URL the user named, else `$GADERNO_URL` (no trailing slash). If neither, probe `http://127.0.0.1:8080/healthz` then `http://127.0.0.1:8765/healthz`. Stop at the first `ok`. If both fail, ask for the listen address.
2. Token: `$GADERNO_TOKEN` or the token the user gave. If set, send `Authorization: Bearer <token>` on every request. Never put the token in the URL. Never log it.
3. `GET $GADERNO_URL/api/agent` and follow that document. It is the route list, curl examples, and status codes. Do not invent endpoints.

## Working loop

1. List or create a notebook. Keep the filename.
2. `POST /api/sessions` with `{"path":"<filename>"}`. Keep `session_id`. Mutators use `/api/sessions/$SID/…` — not the filename.
3. If a session route returns 404, the hub is gone. Open again by path.
4. Insert (`POST …/cells`) or PATCH source. Use cell ids from the response. Do not guess ids.
5. If `kernel.needs_kernel`, bind a name from `GET /api/kernels` via `POST …/kernel`.
6. `POST /api/sessions/$SID/cells/$CID/execute`. Read `stdout`, `stderr`, `ename` from that response.
7. Fix the same cell and execute again. One cell per execute. Prefer small cells.

Tell the user the notebook path and the UI URL (`$GADERNO_URL/n/<path>`) so they can watch.
