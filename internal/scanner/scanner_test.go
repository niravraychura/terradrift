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
	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/niravraychura/terradrift/internal/validation"
)

type fakeRunner struct {
	initErr    error
	initCalled bool
	planExit   int
	planErr    error
	showJSON   []byte
	showErr    error
	planPath   string
	showPath   string
	planMode   terraform.PlanMode
}

func (runner *fakeRunner) Init(ctx context.Context, directory string) error {
	runner.initCalled = true
	return runner.initErr
}

func (runner *fakeRunner) Plan(ctx context.Context, directory string, outputPath string, mode terraform.PlanMode) (int, error) {
	runner.planPath = outputPath
	runner.planMode = mode
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
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
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
	if len(result.Report.ScanID) != 36 {
		t.Fatalf("expected UUID scan ID, got %q", result.Report.ScanID)
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

func TestScanRejectsNegativeTimeoutWithTypedError(t *testing.T) {
	_, err := Scan(context.Background(), Options{Directory: t.TempDir(), Timeout: -time.Second})
	var validationErr *validation.Error
	if !errors.As(err, &validationErr) || validationErr.Field != "scan timeout" {
		t.Fatalf("expected typed timeout validation error, got %v", err)
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
	directory := t.TempDir()
	runner := &fakeRunner{
		planExit: 2,
		showJSON: []byte(`{
			"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"}]}}},
			"resource_changes": [
				{"address":"aws_instance.web","type":"aws_instance","name":"web","change":{"actions":["update"]}}
			]
		}`),
	}

	result, err := Scan(context.Background(), Options{Directory: directory, Runner: runner})
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
	if _, err := os.Stat(filepath.Join(directory, scanLockFilename)); !os.IsNotExist(err) {
		t.Fatalf("expected scan lock to be cleaned up, stat err=%v", err)
	}
}

func TestScanNormalPlanReportsChangesWithoutDrift(t *testing.T) {
	runner := &fakeRunner{planExit: 2, showJSON: []byte(`{
		"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"},{"mode":"managed"}]}}},
		"resource_drift":[{"address":"aws_instance.remote","mode":"managed","change":{"actions":["update"]}}],
		"resource_changes":[{"address":"aws_instance.config","mode":"managed","change":{"actions":["update"]}}]
	}`)}
	result, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: runner, PlanMode: terraform.PlanModeNormal})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if runner.planMode != terraform.PlanModeNormal || result.Outcome != OutcomeChangesDetected || result.Report.Status != report.ScanStatusChangesDetected || result.Report.TotalResourcesChecked != 2 || !result.Report.ResourcesCheckedExact || len(result.Report.ResourceChanges) != 1 || result.Report.ResourceChanges[0].Address != "aws_instance.config" {
		t.Fatalf("unexpected normal result: %#v, mode=%q", result, runner.planMode)
	}
}

func TestPlanModesCanDifferWithTheSamePriorStateCount(t *testing.T) {
	plan := []byte(`{
		"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"},{"mode":"managed"}]}}},
		"resource_drift":[{"address":"aws_instance.remote","mode":"managed","change":{"actions":["update"]}}],
		"resource_changes":[{"address":"aws_instance.config","mode":"managed","change":{"actions":["create"]}}]
	}`)
	refresh, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: &fakeRunner{planExit: 2, showJSON: plan}, PlanMode: terraform.PlanModeRefreshOnly})
	if err != nil {
		t.Fatalf("refresh scan: %v", err)
	}
	normal, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: &fakeRunner{planExit: 2, showJSON: plan}, PlanMode: terraform.PlanModeNormal})
	if err != nil {
		t.Fatalf("normal scan: %v", err)
	}
	if refresh.Report.Status != report.ScanStatusDriftDetected || normal.Report.Status != report.ScanStatusChangesDetected || refresh.Report.TotalResourcesChecked != 2 || normal.Report.TotalResourcesChecked != 2 || refresh.Report.ResourceChanges[0].Address == normal.Report.ResourceChanges[0].Address {
		t.Fatalf("expected distinct mode results with equal inventory: refresh=%#v normal=%#v", refresh.Report, normal.Report)
	}
}

func TestScanRejectsInvalidPlanMode(t *testing.T) {
	_, err := Scan(context.Background(), Options{Directory: t.TempDir(), PlanMode: "apply"})
	if err == nil || !strings.Contains(err.Error(), "plan mode") {
		t.Fatalf("expected invalid plan mode, got %v", err)
	}
}

func TestSecurePlanFileUsesRestrictedPermissionsAndCleansUp(t *testing.T) {
	path, cleanup, err := securePlanFile(t.TempDir())
	if err != nil {
		t.Fatalf("create plan file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 plan file, info=%v err=%v", info, err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup plan file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected plan file to be removed, err=%v", err)
	}
}

func TestScanWithRunnerRejectsExistingLock(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, scanLockFilename), []byte("123"), 0o600); err != nil {
		t.Fatalf("write lock fixture: %v", err)
	}

	result, err := Scan(context.Background(), Options{Directory: directory, Runner: &fakeRunner{}})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected existing lock error, got %v", err)
	}
	if result.Outcome != OutcomeFailed {
		t.Fatalf("expected failed outcome, got %q", result.Outcome)
	}
}

func TestStaleLockGuidanceDetectsMissingProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), scanLockFilename)
	if err := os.WriteFile(path, []byte("999999"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	guidance := staleLockGuidance(path)
	if !strings.Contains(guidance, "999999") {
		t.Fatalf("expected pid in guidance, got %q", guidance)
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

func TestScanRedactsFailedRunnerErrors(t *testing.T) {
	secret := "super-secret-token"
	result, err := Scan(context.Background(), Options{
		Directory: t.TempDir(),
		Runner:    &fakeRunner{initErr: errors.New("token=" + secret)},
	})
	if err == nil {
		t.Fatal("expected init failure")
	}
	if strings.Contains(result.Report.ErrorMessage, secret) || strings.Contains(err.Error(), secret) {
		t.Fatalf("expected errors to be redacted, report=%q error=%q", result.Report.ErrorMessage, err)
	}
	if !strings.Contains(result.Report.ErrorMessage, "[REDACTED]") {
		t.Fatalf("expected a redacted report error, got %q", result.Report.ErrorMessage)
	}
}

func TestScanWithRunnerRejectsUnexpectedPlanExitCode(t *testing.T) {
	result, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: &fakeRunner{planExit: 3}})
	if err == nil || result.Outcome != OutcomeFailed || result.Report.Status != report.ScanStatusFailed {
		t.Fatalf("expected unexpected exit code failure, got %#v, %v", result, err)
	}
}

func TestScanRequiresTerraformFilesWhenRequested(t *testing.T) {
	result, err := Scan(context.Background(), Options{Directory: t.TempDir(), RequireTerraformFiles: true})
	if err == nil || result.Outcome != OutcomeFailed {
		t.Fatalf("expected missing Terraform file failure, got %#v, %v", result, err)
	}
}

func TestParseLockBackend(t *testing.T) {
	for _, name := range []string{"", "local", "LOCAL"} {
		backend, err := ParseLockBackend(name)
		if err != nil {
			t.Fatalf("ParseLockBackend(%q): %v", name, err)
		}
		if _, ok := backend.(LocalFileLockBackend); !ok {
			t.Fatalf("ParseLockBackend(%q): expected LocalFileLockBackend, got %T", name, backend)
		}
	}
	if _, err := ParseLockBackend("redis"); err == nil {
		t.Fatal("expected unknown lock backend to fail")
	}
}

func TestScanSkipInitSkipsRunnerInit(t *testing.T) {
	runner := &fakeRunner{
		planExit: 0,
		showJSON: []byte(`{"prior_state":{"values":{"root_module":{"resources":[]}}},"resource_changes":[]}`),
	}
	result, err := Scan(context.Background(), Options{Directory: t.TempDir(), Runner: runner, SkipInit: true})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if runner.initCalled {
		t.Fatal("expected Init to be skipped")
	}
	if result.Outcome != OutcomeNoDrift {
		t.Fatalf("expected no drift, got %q", result.Outcome)
	}
}
