import { EditorView, basicSetup } from "codemirror";
import { EditorState } from "@codemirror/state";
import { python } from "@codemirror/lang-python";
import { markdown } from "@codemirror/lang-markdown";
import * as Y from "yjs";
import { yCollab } from "y-codemirror.next";
import * as awarenessProtocol from "y-protocols/awareness";
import * as syncProtocol from "y-protocols/sync";
import * as encoding from "lib0/encoding";
import * as decoding from "lib0/decoding";

const COLORS = [
  "#0ea5e9",
  "#8b5cf6",
  "#f59e0b",
  "#10b981",
  "#ef4444",
  "#ec4899",
  "#14b8a6",
  "#f97316",
];

function sourceKey(cellId) {
  return "source:" + cellId;
}

function randomUser() {
  const id =
    Math.random().toString(36).slice(2, 6) +
    Math.random().toString(36).slice(2, 4);
  const color = COLORS[Math.floor(Math.random() * COLORS.length)];
  return {
    name: "user-" + id,
    color,
    colorLight: color + "33",
  };
}

export function createCollabSession() {
  const ydoc = new Y.Doc();
  const awareness = new awarenessProtocol.Awareness(ydoc);
  const user = randomUser();
  awareness.setLocalStateField("user", user);

  const editors = new Map();
  let sendBinary = null;
  let sendJSON = null;
  let connected = false;

  const onDocUpdate = (update, origin) => {
    if (origin === "remote" || !sendBinary) return;
    const encoder = encoding.createEncoder();
    syncProtocol.writeUpdate(encoder, update);
    sendBinary(encoding.toUint8Array(encoder));
  };
  ydoc.on("update", onDocUpdate);

  const onAwareness = ({ added, updated, removed }) => {
    if (!sendJSON) return;
    const changed = added.concat(updated, removed);
    if (changed.length === 0) return;
    const update = awarenessProtocol.encodeAwarenessUpdate(awareness, changed);
    let bin = "";
    for (let i = 0; i < update.length; i++) bin += String.fromCharCode(update[i]);
    sendJSON({ type: "awareness", update: btoa(bin) });
  };
  awareness.on("update", onAwareness);

  function attachTransport(opts) {
    sendBinary = opts.sendBinary;
    sendJSON = opts.sendJSON;
    connected = true;
    const encoder = encoding.createEncoder();
    syncProtocol.writeSyncStep1(encoder, ydoc);
    sendBinary(encoding.toUint8Array(encoder));
    onAwareness({
      added: [awareness.clientID],
      updated: [],
      removed: [],
    });
  }

  function handleSyncMessage(u8) {
    const encoder = encoding.createEncoder();
    const decoder = decoding.createDecoder(u8);
    syncProtocol.readSyncMessage(decoder, encoder, ydoc, "remote");
    if (encoding.length(encoder) > 1 && sendBinary) {
      sendBinary(encoding.toUint8Array(encoder));
    }
  }

  function handleAwarenessB64(b64) {
    try {
      const bin = atob(b64);
      const u8 = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) u8[i] = bin.charCodeAt(i);
      awarenessProtocol.applyAwarenessUpdate(awareness, u8, "remote");
    } catch (_) {}
  }

  function destroyEditors() {
    editors.forEach((v) => v.destroy());
    editors.clear();
  }

  function mountEditors(root) {
    destroyEditors();
    const scope = root || document;
    const seen = new Set();
    scope.querySelectorAll("[data-gaderno-editor]").forEach((host) => {
      const cellId = host.getAttribute("data-cell-id");
      if (!cellId || seen.has(cellId)) {
        // Skip empty/duplicate ids — they share one Y.Text and cause dual-edit bugs.
        host.replaceChildren();
        host.insertAdjacentHTML(
          "beforeend",
          '<p class="text-xs text-error p-2">Invalid cell id (skipped)</p>'
        );
        return;
      }
      seen.add(cellId);
      const lang = host.getAttribute("data-lang") || "python";
      host.replaceChildren();

      const ytext = ydoc.getText(sourceKey(cellId));
      const langExt = lang === "markdown" ? markdown() : python();
      const minH = lang === "markdown" ? 96 : 160;

      const view = new EditorView({
        parent: host,
        state: EditorState.create({
          // Let yCollab own the document; empty seed, sync fills content.
          doc: "",
          extensions: [
            basicSetup,
            langExt,
            EditorView.lineWrapping,
            yCollab(ytext, awareness, { undoManager: false }),
            EditorView.theme({
              "&": { fontSize: "0.8125rem", minHeight: minH + "px" },
              ".cm-scroller": {
                fontFamily:
                  'ui-monospace, "SF Mono", "Cascadia Code", Menlo, Consolas, monospace',
                lineHeight: "1.45",
                minHeight: minH + "px",
              },
              ".cm-content": {
                minHeight: minH - 12 + "px",
                padding: "10px 0",
              },
              "&.cm-focused": {
                outline: "2px solid oklch(0.48 0.14 250)",
              },
            }),
          ],
        }),
      });
      editors.set(cellId, view);
    });

    return {
      getSource(cellId) {
        return ydoc.getText(sourceKey(cellId)).toString();
      },
      focus(cellId) {
        const v = editors.get(cellId);
        if (v) v.focus();
      },
      remount(root) {
        return mountEditors(root);
      },
      destroy: destroyEditors,
    };
  }

  function destroy() {
    destroyEditors();
    awareness.off("update", onAwareness);
    ydoc.off("update", onDocUpdate);
    awareness.setLocalState(null);
    ydoc.destroy();
  }

  return {
    ydoc,
    awareness,
    user,
    attachTransport,
    handleSyncMessage,
    handleAwarenessB64,
    mountEditors,
    destroy,
    get connected() {
      return connected;
    },
  };
}
