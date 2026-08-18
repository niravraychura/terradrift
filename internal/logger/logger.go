// Package logger provides context-aware structured logging helpers.
package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// With returns a child context that carries logger.
func With(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, logger)
}

// From returns the logger stored in ctx, or slog.Default.
func From(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// Info logs an info event with the context logger.
func Info(ctx context.Context, msg string, args ...any) {
	From(ctx).Info(msg, args...)
}

// Error logs an error event with the context logger.
func Error(ctx context.Context, msg string, args ...any) {
	From(ctx).Error(msg, args...)
}
