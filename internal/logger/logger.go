// Package logger provides TerraDrift logging helpers backed by log/slog.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// ParseLevel converts a CLI log-level value to a slog level.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q: supported values are debug, info, warn, error", value)
	}
}

// New creates a structured text logger writing to w.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
