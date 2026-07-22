package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestScanReturnsNoDriftBootstrapResult(t *testing.T) {
	dir := t.TempDir()
	result, err := Scan(context.Background(), Options{Directory: dir})
	if err != nil {
		t.Fatalf("expected scan to succeed: %v", err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs fixture: %v", err)
	}
	if result.Outcome != OutcomeNoDrift {
		t.Fatalf("expected no-drift outcome, got %q", result.Outcome)
	}
	if result.Report.Status != report.ScanStatusNoDrift {
		t.Fatalf("expected no-drift report status, got %q", result.Report.Status)
	}
	if result.Report.Directory != absDir {
		t.Fatalf("expected directory %q, got %q", absDir, result.Report.Directory)
	}
	if result.Report.ResourceChanges == nil {
		t.Fatal("expected empty resource changes, got nil")
	}
}

func TestScanReturnsFailedOutcomeForInvalidDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	result, err := Scan(context.Background(), Options{Directory: missing})
	if err == nil || !strings.Contains(err.Error(), "terraform directory does not exist") {
		t.Fatalf("expected missing directory error, got %v", err)
	}
	if result.Outcome != OutcomeFailed {
		t.Fatalf("expected failed outcome, got %q", result.Outcome)
	}
}

func TestScanHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Scan(ctx, Options{Directory: t.TempDir(), Timeout: time.Minute})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if result.Outcome != OutcomeFailed {
		t.Fatalf("expected failed outcome, got %q", result.Outcome)
	}
}

func TestValidateDirectoryRejectsFilePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "main.tf")
	if err := os.WriteFile(file, []byte("terraform {}\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := ValidateDirectory(file)
	if err == nil || !strings.Contains(err.Error(), "terraform path is not a directory") {
		t.Fatalf("expected file path rejection, got %v", err)
	}
}
