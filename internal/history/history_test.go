package history

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestWriteRejectsSymlinkDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires POSIX permissions")
	}
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "history")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if _, err := Write(link, report.DriftReport{}); err == nil {
		t.Fatal("expected symlink history directory to fail")
	}
}

func TestPruneKeepsNewestReports(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"1.json", "2.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Prune(dir, 1); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2.json")); err != nil {
		t.Fatal("expected newest report")
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

func TestLoadRecentSkipsMalformedReports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2.json"), []byte("not JSON"), 0o600); err != nil {
		t.Fatalf("write malformed report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.json"), []byte(`{"status":"no_drift"}`), 0o600); err != nil {
		t.Fatalf("write valid report: %v", err)
	}

	entries, err := LoadRecent(dir, 1)
	if err != nil {
		t.Fatalf("load recent: %v", err)
	}
	if len(entries) != 1 || entries[0].Report.Status != report.ScanStatusNoDrift {
		t.Fatalf("expected valid report after malformed entry, got %#v", entries)
	}
}

func TestLoadRecentSkipsOversizedReports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2.json"), make([]byte, maxReportBytes+1), 0o600); err != nil {
		t.Fatalf("write oversized report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.json"), []byte(`{"status":"no_drift"}`), 0o600); err != nil {
		t.Fatalf("write valid report: %v", err)
	}

	entries, err := LoadRecent(dir, 1)
	if err != nil {
		t.Fatalf("load recent: %v", err)
	}
	if len(entries) != 1 || entries[0].Report.Status != report.ScanStatusNoDrift {
		t.Fatalf("expected valid report after oversized entry, got %#v", entries)
	}
}
