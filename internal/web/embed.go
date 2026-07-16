package web

import "embed"

// Static holds CSS/JS assets.
//
//go:embed static/*
var Static embed.FS

// Templates holds HTML templates.
//
//go:embed templates/*
var Templates embed.FS
