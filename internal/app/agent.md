# gaderno agent HTTP

Request-response JSON. Do not open the WebSocket.
Do not edit `.ipynb` files on disk while the server is running.

A **session** is the live hub for one notebook (CRDT + optional kernel).
The session id is a UUID. It is not the filename. Open by path once, then put the id in the URL.

If a session route returns 404, the hub is gone (server restart). Open again by path.

## Auth

If the server has a shared token, send it on every request:

```text
Authorization: Bearer <token>
```

Use `$GADERNO_TOKEN` or the token the user gave you. Never put the token in the URL. Never log it.

No token configured: send no auth header.

## Discover

```bash
curl -fsS "$GADERNO_URL/healthz"          # ok
curl -fsS "$GADERNO_URL/api/version"      # version string
```

`$GADERNO_URL` has no trailing slash (example: `http://127.0.0.1:8080`).

## Open a session from a filename

List notebooks, or create one. Then open (or join) the live session:

```bash
curl -fsS "$GADERNO_URL/api/notebooks"
# {"notebooks":["analysis.ipynb"]}

curl -fsS -X POST "$GADERNO_URL/api/notebooks" \
  -H 'Content-Type: application/json' \
  -d '{"name":"scratch"}'
# {"path":"scratch.ipynb"}

curl -fsS -X POST "$GADERNO_URL/api/sessions" \
  -H 'Content-Type: application/json' \
  -d '{"path":"scratch.ipynb"}'
```

```json
{
  "session_id": "3f2c0a1e-…",
  "path": "scratch.ipynb",
  "kernel": {"phase": "bound", "bound_name": "python3", "needs_kernel": false},
  "cells": [
    {
      "id": "a1b2c3d4",
      "type": "code",
      "source": "print(1)",
      "execution_count": 1,
      "stdout": "1\n",
      "text": "<Figure size ...>",
      "omitted": ["image/png"]
    }
  ]
}
```

Remember `session_id`. Every mutator uses it as a route param.

Find a live session by filename:

```bash
curl -fsS "$GADERNO_URL/api/sessions"
# {"sessions":[{"id":"3f2c0a1e-…","path":"scratch.ipynb","kernel":{…}}]}
```

Read the compact notebook again:

```bash
curl -fsS "$GADERNO_URL/api/sessions/$SID"
```

`omitted` lists mime types dropped from the compact view (images, HTML). Open the notebook in a browser to see them.

Kernel `phase`: `needs_kernel` | `bound` | `starting` | `ready` | `busy` | `dead`.
Exec is blocked while `needs_kernel` is true. Do not bind or switch a kernel.
Tell the user to pick one in the UI (`<url>/n/<path>`, session status control).

Full nbformat (`GET /api/notebooks/<path>`) includes base64 images. Do not load it into context.

## Cells

`$SID` is the session id. `$CID` is the cell id from the compact notebook.
Every mutation returns the compact notebook (same shape as `GET /api/sessions/$SID`).

Insert (`index` omitted → append; `type` omitted → `code`):

```bash
curl -fsS -X POST "$GADERNO_URL/api/sessions/$SID/cells" \
  -H 'Content-Type: application/json' \
  -d '{"index":1,"type":"code","source":"print(1+1)"}'
```

Replace source and/or type (`source` may be `""` to clear):

```bash
curl -fsS -X PATCH "$GADERNO_URL/api/sessions/$SID/cells/$CID" \
  -H 'Content-Type: application/json' \
  -d '{"source":"print(42)"}'

curl -fsS -X PATCH "$GADERNO_URL/api/sessions/$SID/cells/$CID" \
  -H 'Content-Type: application/json' \
  -d '{"type":"markdown"}'
```

Move (`index` is the destination, 0-based):

```bash
curl -fsS -X POST "$GADERNO_URL/api/sessions/$SID/cells/$CID/move" \
  -H 'Content-Type: application/json' \
  -d '{"index":0}'
```

Delete:

```bash
curl -fsS -X DELETE "$GADERNO_URL/api/sessions/$SID/cells/$CID"
```

Unknown session or cell → 404. Invalid `type` → 400. Types: `code` | `markdown` | `raw`.

## Execute

Waits until the cell finishes (up to 2 minutes) and returns the result. Optional `source` is written first, then run.

The kernel is whoever the user already bound (notebook metadata or the UI chooser). There is no agent bind route. A `kernel` field in the execute body is ignored.

```bash
curl -fsS -X POST "$GADERNO_URL/api/sessions/$SID/cells/$CID/execute" \
  -H 'Content-Type: application/json' \
  -d '{"source":"print(1+1)"}'
```

```json
{
  "msg_id": "...",
  "status": "ok",
  "execution_count": 1,
  "stdout": "2\n",
  "stderr": "",
  "ename": "",
  "evalue": ""
}
```

`status` is `ok` | `error` | `abort`. On error read `ename`, `evalue`, and optional `traceback`.

No kernel bound (`needs_kernel`) → 400. Tell the user to pick a kernel in the UI, then execute again. Spawn failure → 502.

Stop a running execute:

```bash
curl -fsS -X POST "$GADERNO_URL/api/sessions/$SID/interrupt"
```

No live kernel process → 409.

## Save

Disk flush is debounced after mutations. Force a write:

```bash
curl -fsS -X POST "$GADERNO_URL/api/sessions/$SID/save"
```

## Status codes

| Code | Meaning |
|------|---------|
| 200 | OK (execute result, compact notebook, lists) |
| 201 | Notebook created |
| 204 | Save ok |
| 400 | Bad input, or no kernel selected |
| 401 | Shared token missing or wrong |
| 404 | Session, notebook, or cell not found |
| 409 | Kernel process not started (interrupt with no process) |
| 502 | Kernel spawn failed |

## Working loop

1. List notebooks or create one. Note the filename.
2. `POST /api/sessions` with that path. Keep `session_id`.
3. Insert or PATCH source. Use cell ids from the response. Do not guess ids.
4. If `needs_kernel`, stop and tell the user to pick a kernel in the UI. Do not bind one.
5. `POST /api/sessions/$SID/cells/$CID/execute`. Read `stdout` / `stderr` / `ename`.
6. Fix the same cell and execute again. Do not rewrite the whole notebook.

One cell per execute. Prefer small cells. Markdown cells do not run.
