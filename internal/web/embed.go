package web

import "embed"

// Static holds CSS/JS assets (app client, editor bundle, vendored daisyUI/Tailwind).
//
//go:embed static/*
var Static embed.FS
