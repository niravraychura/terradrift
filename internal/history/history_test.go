package history

import (
	"os"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestWriteAndLoadRecent(t *testing.T) {
	dir := t.TempDir()
	path, err := Write(dir, report.DriftReport{Status: report.ScanStatusNoDrift, Directory: "[REDACTED]"})
	if err != nil {
		t.Fatalf("write history: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat history file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected history file permissions 0600, got %o", got)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat history dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("expected history dir permissions 0700, got %o", got)
	}
	entries, err := LoadRecent(dir, 10)
	if err != nil {
		t.Fatalf("load recent: %v", err)
	}
	if len(entries) != 1 || entries[0].Report.Status != report.ScanStatusNoDrift {
		t.Fatalf("unexpected history entries: %#v", entries)
	}
}

func TestWriteStoresRedactedReportAsProvided(t *testing.T) {
	dir := t.TempDir()
	path, err := Write(dir, report.DriftReport{Status: report.ScanStatusNoDrift, Directory: "[REDACTED]"})
	if err != nil {
		t.Fatalf("write history: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("expected redacted directory to be persisted, got %q", data)
	}
}
