// Package log configures the application logger.
//
// Stardust writes human-readable status to stderr so that stdout stays clean
// for data (markdown or JSON) consumed by agents and pipes.
package log

import (
	"log/slog"
	"os"
)

// New returns a slog.Logger writing text-formatted status to stderr. When debug
// is true the level is lowered to Debug, otherwise Info.
func New(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
