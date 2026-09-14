/**
 * Copy published CSS/JS runtime assets into the embed tree.
 * Versions come from package.json / bun.lock (Renovate-friendly).
 */
import { copyFileSync, mkdirSync, statSync } from "fs";
import { join } from "path";

const root = join(import.meta.dir, "..");
const outDir = join(root, "internal/web/static/vendor");

const copies: { from: string; to: string }[] = [
  {
    from: "node_modules/daisyui/daisyui.css",
    to: "daisyui.css",
  },
  {
    from: "node_modules/@tailwindcss/browser/dist/index.global.js",
    to: "tailwind-browser.js",
  },
];

mkdirSync(outDir, { recursive: true });

for (const { from, to } of copies) {
  const src = join(root, from);
  const dst = join(outDir, to);
  try {
    statSync(src);
  } catch {
    console.error(`vendor-web: missing ${from} — run bun install`);
    process.exit(1);
  }
  copyFileSync(src, dst);
  const n = statSync(dst).size;
  console.log(`vendor-web: ${to} (${n} bytes)`);
}
