---
name: gaderno
description: >
  Talk to a running gaderno notebook server over HTTP (curl). Use when the
  user wants to create, edit, or run notebooks on gaderno, says /gaderno,
  mentions gaderno cells or kernels, pastes a gaderno URL or /SKILL.md, or
  asks an agent to work in a live notebook instead of editing .ipynb files
  by hand.
---

# gaderno

The live document is on the server. The instance skill is `GET /SKILL.md` (token required when the server has one). That file is the procedure. This file only gets you there.

## Connect

1. If the user pasted an origin, a `GET …/SKILL.md` URL, and/or a Bearer token, use those. Do not ask again.
2. Otherwise ask where the gaderno instance is (listen URL, no trailing slash) and whether a shared token is set. Do not guess localhost, default ports, or `$GADERNO_URL`. Do not probe.
3. `GET <origin>/SKILL.md`. If they gave a token, send `Authorization: Bearer <token>`. Follow that document. Do not invent endpoints.
4. Never put the token in the URL. Never log it.
