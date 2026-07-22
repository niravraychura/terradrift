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

type fakeRunner struct {
	initErr    error
	planExit   int
	planErr    error
	showJSON   []byte
	showErr    error
	planPath   string
	showPath   string
	cleanupDir string
}

func (runner *fakeRunner) Init(ctx context.Context, directory string) error {
	return runner.initErr
}

func (runner *fakeRunner) PlanRefreshOnly(ctx context.Context, directory string, outputPath string) (int, error) {
	runner.planPath = outputPath
	runner.cleanupDir = directory
	return runner.planExit, runner.planErr
}

func (runner *fakeRunner) ShowJSON(ctx context.Context, directory string, planPath string) ([]byte, error) {
	runner.showPath = planPath
	return runner.showJSON, runner.showErr
}

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

func TestValidateWorkspaceRootAllowsDirectoryInsideRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "env")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := ValidateWorkspaceRoot(dir, root); err != nil {
		t.Fatalf("expected directory inside root to be accepted: %v", err)
	}
}

func TestValidateWorkspaceRootRejectsDirectoryOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := ValidateWorkspaceRoot(outside, root); err == nil {
		t.Fatal("expected outside directory to be rejected")
	}
}

func TestScanWithRunnerReturnsDriftDetected(t *testing.T) {
	runner := &fakeRunner{
		planExit: 2,
		showJSON: []byte(`{
			"resource_changes": [
				{"address":"aws_instance.web","type":"aws_instance","name":"web","change":{"actions":["update"]}}
			]
		}`),
	}

	result, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: runner})
	if err != nil {
		t.Fatalf("expected scan to succeed: %v", err)
	}
	if result.Outcome != OutcomeDriftDetected {
		t.Fatalf("expected drift outcome, got %q", result.Outcome)
	}
	if result.Report.TotalResourcesChecked != 1 || result.Report.TotalChangedResources != 1 {
		t.Fatalf("unexpected resource counts: %#v", result.Report)
	}
	if runner.planPath == "" || runner.showPath != runner.planPath {
		t.Fatalf("expected secure plan path to be passed through runner, plan=%q show=%q", runner.planPath, runner.showPath)
	}
	if _, err := os.Stat(runner.planPath); !os.IsNotExist(err) {
		t.Fatalf("expected secure plan file to be cleaned up, stat err=%v", err)
	}
}

func TestScanWithRunnerReturnsFailedOutcomeForPlanError(t *testing.T) {
	runner := &fakeRunner{planExit: 1}

	result, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: runner})
	if err == nil {
		t.Fatal("expected plan failure error")
	}
	if result.Outcome != OutcomeFailed || result.Report.Status != report.ScanStatusFailed {
		t.Fatalf("expected failed result, got %#v", result)
	}
	if result.Report.ErrorMessage == "" {
		t.Fatal("expected error message to be recorded")
	}
}
