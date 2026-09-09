---
name: gaderno
description: >
  Operate this live gaderno notebook server over HTTP (curl). You fetched
  this file from the instance. Use when creating, editing, or running
  notebooks here.
---

# gaderno

You fetched this from the instance. The base URL is this document's origin (no trailing slash).

Use the same `Authorization: Bearer` you used to GET this file on every request. Never put the token in the URL. Never log it. If this file loaded without a token, send no auth header.

Do not open the WebSocket. Do not rewrite `.ipynb` files on disk.

`GET /api/agent` on this origin is the route list, curl examples, and status codes. Follow it. Do not invent endpoints.

## Working loop

1. List or create a notebook. Keep the filename.
2. `POST /api/sessions` with `{"path":"<filename>"}`. Keep `session_id`. Mutators use `/api/sessions/$SID/…` — not the filename.
3. If a session route returns 404, the hub is gone. Open again by path.
4. Insert (`POST …/cells`) or PATCH source. Use cell ids from the response. Do not guess ids.
5. If `kernel.needs_kernel`, stop. Tell the user to pick a kernel in the UI. Do not bind or switch a kernel.
6. `POST /api/sessions/$SID/cells/$CID/execute`. Read `stdout`, `stderr`, `ename` from that response.
7. Fix the same cell and execute again. One cell per execute. Prefer small cells.
8. Need a Python package? Run `!uv pip install <pkg>` in a code cell. Do not use bare `pip`, `conda`, or the host package manager.
9. `POST /api/sessions/$SID/chat` with `{"text":"…"}` so they see you in the session chat panel. `GET …/chat` reads the RAM tail (what they typed there). Author is always `agent`.

Tell the user the notebook path and the UI URL (`<origin>/n/<path>`) so they can watch.
