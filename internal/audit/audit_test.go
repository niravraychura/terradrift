package audit

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestEnrichAttachesEvents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires POSIX shell")
	}
	command := filepath.Join(t.TempDir(), "audit")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '{\"resource_events\":[{\"address\":\"aws_instance.web\",\"events\":[{\"provider\":\"aws\",\"actor\":\"alice\",\"summary\":\"updated instance\"}]}]}'\n"), 0o700); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	scanReport, err := Enrich(context.Background(), Options{Command: command}, report.DriftReport{ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web"}}})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(scanReport.ResourceChanges[0].AuditEvents) != 1 || scanReport.ResourceChanges[0].AuditEvents[0].Actor != "alice" {
		t.Fatalf("unexpected events: %#v", scanReport.ResourceChanges)
	}
}

func TestEnrichRejectsOversizedInput(t *testing.T) {
	_, err := Enrich(context.Background(), Options{Command: "false"}, report.DriftReport{ErrorMessage: strings.Repeat("x", maxInputBytes)})
	if err == nil || !strings.Contains(err.Error(), "audit input exceeds") {
		t.Fatalf("expected oversized input error, got %v", err)
	}
}
