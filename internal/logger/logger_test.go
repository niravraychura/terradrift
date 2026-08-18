package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWithAndFrom(t *testing.T) {
	var buffer bytes.Buffer
	custom := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := With(context.Background(), custom)
	Info(ctx, "scan started", "directory", "terraform/prod")
	if !strings.Contains(buffer.String(), "scan started") || !strings.Contains(buffer.String(), "terraform/prod") {
		t.Fatalf("expected contextual log, got %q", buffer.String())
	}
	if From(context.Background()) == nil {
		t.Fatal("expected default logger")
	}
}
