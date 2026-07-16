# gaderno web UI

## Intent
Notebook product surface: workspace list + notebook editor. Feel **Jupyter-legible**, layout **contapila-dense**.

## Color (OKLCH, restrained)
| Role | Light |
|------|--------|
| base-100 | oklch(1 0 0) pure white |
| base-200 | oklch(0.97 0.005 250) cool wash for code wells |
| base-300 | oklch(0.90 0.01 250) borders |
| ink | oklch(0.22 0.02 250) |
| muted | oklch(0.45 0.02 250) |
| primary | oklch(0.48 0.14 250) cobalt |
| primary-content | oklch(0.98 0.01 250) |
| success | oklch(0.48 0.12 155) |
| error | oklch(0.52 0.18 25) |
| warn | oklch(0.72 0.12 85) |
| running | oklch(0.55 0.12 230) |

## Typography
- system-ui for chrome; ui-monospace for code/outputs
- Dense: topbar ~0.8125rem, body 0.875rem, code 0.8125rem
- Tabular nums on execution counts

## Layout
```
┌─ sticky topbar (h ~2.25rem) ─────────────────────────────┐
│ gaderno · path.ipynb     kernel · status    [Save]       │
├──────────────────────────────────────────────────────────┤
│ full-bleed cells (pad 0.5–0.75rem)                       │
│  [In]  code well + Run                                   │
│  [Out] stream / error                                    │
├─ chat drawer compact (optional strip) ───────────────────┤
```

## Density (from contapila)
- Sticky chrome, minimal vertical rhythm
- No max-width card sea; content uses available width
- Cells separated by hairline, not large rounded cards
- radius ≤ 0.25rem

## Motion
150–200ms status only; prefers-reduced-motion respected.
