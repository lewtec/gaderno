// Package tools holds module-level go:generate directives.
// Working directory for these is the module root (this directory).
package tools

// Vendor CSS/JS from npm (daisyUI + Tailwind browser) and rebuild the editor bundle.
//go:generate bun install --frozen-lockfile
//go:generate bun run vendor:web
//go:generate bun run build:js

// Typed HTML components.
//go:generate go tool templ generate
