package log

import (
	"log/slog"
	"os"
)

// New returns a text slog logger to stderr.
func New() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
