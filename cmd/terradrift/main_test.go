package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/niravraychura/terradrift/internal/config"
	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/notify"
	"github.com/niravraychura/terradrift/internal/parser"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/niravraychura/terradrift/internal/terraform"
)

func executeCommand(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestVersionFlag(t *testing.T) {
	stdout, _, err := executeCommand("--version")
	if err != nil {
		t.Fatalf("expected --version to succeed: %v", err)
	}
	if !strings.Contains(stdout, version) {
		t.Fatalf("expected version output to include %q, got %q", version, stdout)
	}
	if !strings.Contains(stdout, "terradrift") {
		t.Fatalf("expected version output to include binary name, got %q", stdout)
	}
}

func TestScanDefaultsToCurrentDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}

	stdout, _, err := executeCommand("scan")
	if err != nil {
		t.Fatalf("expected current directory default to be valid, got %v", err)
	}
	if !strings.Contains(stdout, "Terraform directory: "+wd) {
		t.Fatalf("expected stdout to include current directory %q, got %q", wd, stdout)
	}
}

func TestReadLimitedFileRejectsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, make([]byte, 2), 0o600); err != nil {
		t.Fatalf("write report fixture: %v", err)
	}
	if _, err := ioutil.ReadLimitedFile(path, 1); err == nil {
		t.Fatal("expected oversized file to fail")
	}
}

func TestScanAllLoadsRelativeManifestRoots(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"development", "production"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatalf("create root fixture: %v", err)
		}
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("# Terraform roots\ndevelopment\nproduction\n"), 0o600); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}

	stdout, _, err := executeCommand("scan-all", "--manifest", manifest, "--output", "json", "--concurrency", "1")
	if err != nil {
		t.Fatalf("expected multi-root scan to succeed: %v", err)
	}
	var aggregate multiScanReport
	if err := json.Unmarshal([]byte(stdout), &aggregate); err != nil {
		t.Fatalf("expected aggregate JSON: %v", err)
	}
	if aggregate.Status != multiScanStatusComplete || aggregate.TotalRoots != 2 || aggregate.FailedRoots != 0 || len(aggregate.Roots) != 2 {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
}

func TestScanAllJSONManifestPerRootPlanMode(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"development", "production"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatalf("create root fixture: %v", err)
		}
	}
	manifest := filepath.Join(root, "roots.json")
	payload := `{
  "version": 1,
  "roots": [
    {"directory": "development", "plan_mode": "refresh-only"},
    {"directory": "production", "plan_mode": "normal", "workspace": "prod", "var_files": ["prod.tfvars"]}
  ]
}`
	if err := os.WriteFile(manifest, []byte(payload), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	stdout, _, err := executeCommand("scan-all", "--manifest", manifest, "--output", "json", "--concurrency", "1")
	if err != nil {
		t.Fatalf("scan-all json manifest: %v", err)
	}
	var aggregate multiScanReport
	if err := json.Unmarshal([]byte(stdout), &aggregate); err != nil {
		t.Fatalf("decode aggregate: %v", err)
	}
	if aggregate.TotalRoots != 2 || aggregate.FailedRoots != 0 {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
	byDir := map[string]multiScanRoot{}
	for _, item := range aggregate.Roots {
		byDir[filepath.Base(item.Directory)] = item
	}
	if byDir["development"].Report.PlanMode != "refresh-only" {
		t.Fatalf("development plan mode = %q", byDir["development"].Report.PlanMode)
	}
	if byDir["production"].Report.PlanMode != "normal" || byDir["production"].Report.Status != report.ScanStatusNoChanges {
		t.Fatalf("production root = %#v", byDir["production"])
	}
}

func TestScanAllJSONManifestProfileRequiresConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "production"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := filepath.Join(root, "roots.json")
	if err := os.WriteFile(manifest, []byte(`{"version":1,"roots":[{"directory":"production","profile":"production"}]}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	stdout, _, err := executeCommand("scan-all", "--manifest", manifest, "--output", "json")
	if err == nil {
		t.Fatalf("expected profile without config to fail, stdout=%q", stdout)
	}
	if !strings.Contains(err.Error(), "1 roots") {
		t.Fatalf("expected multi-root failure, got %v", err)
	}
}

func TestLoadJSONManifestRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roots.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"roots":[{"directory":".","extra":true}]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := loadScanManifest(path); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestResolveRootOptionsAppliesOverrides(t *testing.T) {
	runner := terraform.NewCLIRunner("terraform")
	runner.Workspace = "global"
	runner.VarFiles = []string{"global.tfvars"}
	options := scanner.Options{Runner: runner, PlanMode: terraform.PlanModeRefreshOnly}
	resolved, err := resolveRootOptions(manifestRoot{
		Directory: "/tmp/root",
		PlanMode:  "normal",
		Workspace: "prod",
		VarFiles:  []string{"prod.tfvars"},
		Vars:      []string{"env=prod"},
	}, rootDefaults{PlanMode: "refresh-only", Workspace: "global", VarFiles: []string{"global.tfvars"}}, options)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.PlanMode != terraform.PlanModeNormal || resolved.Directory != "/tmp/root" {
		t.Fatalf("unexpected options: %#v", resolved)
	}
	cli, ok := resolved.Runner.(terraform.CLIRunner)
	if !ok || cli.Workspace != "prod" || len(cli.VarFiles) != 1 || cli.VarFiles[0] != "prod.tfvars" || len(cli.Vars) != 1 {
		t.Fatalf("unexpected runner: %#v", resolved.Runner)
	}
}

func TestScanAllAcceptsNormalPlanMode(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "production")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create root: %v", err)
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("production\n"), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	stdout, _, err := executeCommand("scan-all", "--manifest", manifest, "--plan-mode", "normal", "--output", "json")
	if err != nil {
		t.Fatalf("scan-all: %v", err)
	}
	var aggregate multiScanReport
	if err := json.Unmarshal([]byte(stdout), &aggregate); err != nil || len(aggregate.Roots) != 1 || aggregate.Roots[0].Report.PlanMode != "normal" || aggregate.Roots[0].Report.Status != report.ScanStatusNoChanges {
		t.Fatalf("unexpected normal scan-all report: %#v, err=%v", aggregate, err)
	}
}

func TestMultiScanStatus(t *testing.T) {
	for _, test := range []struct {
		total, drifted, failed int
		want                   multiScanStatus
	}{
		{total: 2, want: multiScanStatusComplete},
		{total: 2, drifted: 1, want: multiScanStatusDriftDetected},
		{total: 2, failed: 1, want: multiScanStatusPartial},
		{total: 2, failed: 2, want: multiScanStatusFailed},
	} {
		if got := multiScanStatusFor(test.total, test.drifted, 0, test.failed); got != test.want {
			t.Fatalf("status(%d, %d, %d) = %q, want %q", test.total, test.drifted, test.failed, got, test.want)
		}
	}
}

func TestMultiScanStatusReportsNormalChanges(t *testing.T) {
	if got := multiScanStatusFor(1, 0, 1, 0); got != multiScanStatusChangesDetected {
		t.Fatalf("expected normal changes status, got %q", got)
	}
}

func TestScanAllReportsPartialOutcome(t *testing.T) {
	valid := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	aggregate := scanAll(context.Background(), scanAllParams{
		Specs:       []manifestRoot{{Directory: valid}, {Directory: missing}},
		Defaults:    rootDefaults{PlanMode: "refresh-only"},
		Concurrency: 1,
	})
	if aggregate.Status != multiScanStatusPartial || aggregate.FailedRoots != 1 {
		t.Fatalf("expected partial aggregate, got %#v", aggregate)
	}
}

func TestIncrementalRootsRetriesOnlyUnhealthyRoots(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state := multiScanReport{Roots: []multiScanRoot{
		{Directory: "healthy", Report: report.DriftReport{Status: report.ScanStatusNoDrift}},
		{Directory: "drifted", Report: report.DriftReport{Status: report.ScanStatusDriftDetected}},
		{Directory: "failed", Error: "scan failed"},
	}}
	if err := writeIncrementalState(statePath, state); err != nil {
		t.Fatalf("write state: %v", err)
	}
	roots, err := incrementalRoots(statePath, []string{"healthy", "drifted", "failed", "new"})
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if got, want := strings.Join(roots, ","), "drifted,failed,new"; got != want {
		t.Fatalf("incremental roots = %q, want %q", got, want)
	}
}

func TestRunDeliveriesReturnsEveryFailure(t *testing.T) {
	err := runDeliveries([]deliveryTask{
		{name: "one", run: func() error { return errors.New("first") }},
		{name: "two", run: func() error { return errors.New("second") }},
	})
	if err == nil || !strings.Contains(err.Error(), "one delivery") || !strings.Contains(err.Error(), "two delivery") {
		t.Fatalf("expected labelled delivery errors, got %v", err)
	}
}

func TestEnrichReportRunsIndependentAdaptersConcurrently(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixtures require POSIX")
	}
	directory := t.TempDir()
	costCommand := filepath.Join(directory, "cost")
	auditCommand := filepath.Join(directory, "audit")
	if err := os.WriteFile(costCommand, []byte("#!/bin/sh\nsleep 0.2\nprintf '{\"resource_costs\":[{\"address\":\"aws_instance.web\",\"monthly_delta\":\"$1\"}]}'\n"), 0o700); err != nil {
		t.Fatalf("write cost fixture: %v", err)
	}
	if err := os.WriteFile(auditCommand, []byte("#!/bin/sh\nsleep 0.2\nprintf '{\"resource_events\":[{\"address\":\"aws_instance.web\",\"events\":[{\"provider\":\"aws\",\"actor\":\"operator\",\"occurred_at\":\"2026-01-01T00:00:00Z\",\"summary\":\"changed\"}]}]}'\n"), 0o700); err != nil {
		t.Fatalf("write audit fixture: %v", err)
	}
	enriched, err := enrichReport(context.Background(), report.DriftReport{ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web"}}}, costCommand, nil, auditCommand, nil)
	if err != nil || enriched.ResourceChanges[0].CostImpact != "$1" || len(enriched.ResourceChanges[0].AuditEvents) != 1 {
		t.Fatalf("unexpected concurrent enrichment: %#v err=%v", enriched, err)
	}
}

func TestDiscoverTerraformRootsHonorsPatterns(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"included", "excluded", ".terraform/cache"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatalf("create root fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, directory, "main.tf"), []byte("terraform {}"), 0o600); err != nil {
			t.Fatalf("write Terraform fixture: %v", err)
		}
	}

	directories, err := discoverTerraformRoots(root, []string{"included"}, []string{"excluded"})
	if err != nil {
		t.Fatalf("discover roots: %v", err)
	}
	if len(directories) != 1 || directories[0] != filepath.Join(root, "included") {
		t.Fatalf("unexpected discovered roots: %#v", directories)
	}
}

func TestHistoryHandlerServesReadOnlyReports(t *testing.T) {
	historyDir := t.TempDir()
	if _, err := history.Write(historyDir, report.DriftReport{Status: report.ScanStatusNoDrift}); err != nil {
		t.Fatalf("write history fixture: %v", err)
	}
	handler := newHistoryHandler(historyDir, 10)

	for _, path := range []string{"/reports", "/"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected %s to succeed, got %d", path, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/reports", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected writes to be rejected, got %d", recorder.Code)
	}
}

func TestDashboardIndexWritesHistory(t *testing.T) {
	historyDir := t.TempDir()
	if _, err := history.Write(historyDir, report.DriftReport{Directory: "terraform/prod", Status: report.ScanStatusNoDrift}); err != nil {
		t.Fatalf("write history fixture: %v", err)
	}
	output := filepath.Join(t.TempDir(), "index.html")
	_, _, err := executeCommand("dashboard-index", "--history-dir", historyDir, "--output", output)
	if err != nil {
		t.Fatalf("expected dashboard index to succeed: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read dashboard index: %v", err)
	}
	if !strings.Contains(string(data), "terraform/prod") {
		t.Fatalf("expected dashboard index to contain history, got %q", data)
	}
}

func TestApproveCreatesSecureArtifact(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.json")
	data, err := json.Marshal(report.DriftReport{Status: report.ScanStatusDriftDetected, ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}}}})
	if err != nil {
		t.Fatalf("encode report fixture: %v", err)
	}
	if err := os.WriteFile(reportPath, data, 0o600); err != nil {
		t.Fatalf("write report fixture: %v", err)
	}
	approvalPath := filepath.Join(t.TempDir(), "approval.json")
	_, _, err = executeCommand("approve", "--report", reportPath, "--owner", "platform", "--reason", "approved maintenance", "--expires-at", time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "--output", approvalPath)
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}
	info, err := os.Stat(approvalPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("expected secure approval artifact, info=%v err=%v", info, err)
	}
}

func TestScanHelpIncludesSafetyFlags(t *testing.T) {
	stdout, _, err := executeCommand("scan", "--help")
	if err != nil {
		t.Fatalf("show scan help: %v", err)
	}
	for _, flag := range []string{"--terraform-exec", "--plan-mode", "--redact-paths", "--workspace-root", "--audit-command", "--approval-file"} {
		if !strings.Contains(stdout, flag) {
			t.Fatalf("expected scan help to contain %q", flag)
		}
	}
	for _, section := range []string{"Flag groups:", "Core:", "Delivery:", "Enrichment:"} {
		if !strings.Contains(stdout, section) {
			t.Fatalf("expected scan help to contain %q", section)
		}
	}
}

func TestScanAllHelpIncludesPlanMode(t *testing.T) {
	stdout, _, err := executeCommand("scan-all", "--help")
	if err != nil || !strings.Contains(stdout, "--plan-mode") {
		t.Fatalf("expected scan-all plan mode help, stdout=%q err=%v", stdout, err)
	}
}

func TestWriteDashboardRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires POSIX permissions")
	}
	target := filepath.Join(t.TempDir(), "target.html")
	path := filepath.Join(t.TempDir(), "dashboard.html")
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := writeDashboard(path, report.DriftReport{}, nil); err == nil {
		t.Fatal("expected symlink dashboard path to fail")
	}
}

func TestNormalizeOutputPathRejectsSymlinkParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires POSIX permissions")
	}
	parent := filepath.Join(t.TempDir(), "parent")
	if err := os.Symlink(t.TempDir(), parent); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if _, err := normalizeOutputPath(filepath.Join(parent, "output.json")); err == nil {
		t.Fatal("expected symlink parent to fail")
	}
}

func TestShouldCreatePersistentIssue(t *testing.T) {
	current := report.DriftReport{Directory: "terraform/prod", Status: report.ScanStatusDriftDetected, ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"}}}
	entries := []history.Entry{{Report: current}, {Report: current}}
	if !shouldCreatePersistentIssue(current, entries, 3) {
		t.Fatal("expected third matching scan to create an issue")
	}
	entries = append(entries, history.Entry{Report: current})
	if shouldCreatePersistentIssue(current, entries, 3) {
		t.Fatal("expected issue creation to occur only once per persistent sequence")
	}
}

func TestRedactedHistoryKeepsRootIdentity(t *testing.T) {
	current := report.DriftReport{RootID: "root-a", Directory: "[REDACTED]", Status: report.ScanStatusDriftDetected, ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}}}}
	other := report.DriftReport{RootID: "root-b", Directory: "[REDACTED]", Status: report.ScanStatusDriftDetected, ResourceChanges: current.ResourceChanges}
	if previous := previousReportForRoot([]history.Entry{{Report: other}}, current); previous.RootID != "" {
		t.Fatalf("expected no cross-root history match, got %#v", previous)
	}
	if shouldCreatePersistentIssue(current, []history.Entry{{Report: other}}, 2) {
		t.Fatal("expected different redacted roots to remain independent")
	}
}

func TestScanRejectsNonexistentDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, _, err := executeCommand("scan", "--directory", missing)
	if err == nil || !strings.Contains(err.Error(), "terraform directory does not exist") {
		t.Fatalf("expected nonexistent directory error, got %v", err)
	}
}

func TestScanRejectsFilePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "main.tf")
	if err := os.WriteFile(file, []byte("terraform {}\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, _, err := executeCommand("scan", "-d", file)
	if err == nil || !strings.Contains(err.Error(), "terraform path is not a directory") {
		t.Fatalf("expected not directory error, got %v", err)
	}
}

func TestScanValidDirectoryTableOutput(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := executeCommand("scan", "-d", dir)
	if err != nil {
		t.Fatalf("expected valid directory, got %v", err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs fixture: %v", err)
	}
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	for _, want := range []string{
		"TerraDrift scan initialized",
		"Status: no_drift",
		"Terraform directory: " + absDir,
		"Resources checked: 0",
		"Changed resources: 0",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
		}
	}
}

func TestScanRedactsDirectoryWhenRequested(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := executeCommand("scan", "-d", dir, "--redact-paths")
	if err != nil {
		t.Fatalf("expected valid directory, got %v", err)
	}
	if strings.Contains(stdout, dir) {
		t.Fatalf("expected directory to be redacted from stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "Terraform directory: [REDACTED]") {
		t.Fatalf("expected redacted directory marker, got %q", stdout)
	}
}

func TestScanValidDirectoryJSONOutput(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := executeCommand("scan", "-d", dir, "--output", "json")
	if err != nil {
		t.Fatalf("expected valid directory, got %v", err)
	}

	var scanReport report.DriftReport
	if err := json.Unmarshal([]byte(stdout), &scanReport); err != nil {
		t.Fatalf("expected valid JSON output, got %v: %q", err, stdout)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs fixture: %v", err)
	}
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	if scanReport.Directory != absDir {
		t.Fatalf("expected directory %q, got %q", absDir, scanReport.Directory)
	}
	if scanReport.Status != report.ScanStatusNoDrift {
		t.Fatalf("expected no drift status, got %q", scanReport.Status)
	}
	if scanReport.ResourceChanges == nil {
		t.Fatal("expected resource changes to be an empty slice, got nil")
	}
}

func TestScanNormalModeJSONOutput(t *testing.T) {
	stdout, _, err := executeCommand("scan", "-d", t.TempDir(), "--plan-mode", "normal", "--output", "json")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var scanReport report.DriftReport
	if err := json.Unmarshal([]byte(stdout), &scanReport); err != nil || scanReport.PlanMode != "normal" || scanReport.Status != report.ScanStatusNoChanges {
		t.Fatalf("unexpected normal report: %#v, err=%v", scanReport, err)
	}
}

func TestWriteScanReportTableIncludesAttributeDiffs(t *testing.T) {
	var output bytes.Buffer
	err := writeScanReport(&output, report.DriftReport{
		Status:                report.ScanStatusChangesDetected,
		PlanMode:              "normal",
		ScanID:                "scan-1",
		Directory:             "/tmp/terraform",
		TotalResourcesChecked: 10,
		TotalChangedResources: 1,
		ResourceChanges: []report.ResourceChange{{
			Address:      "aws_lb.main",
			Actions:      []string{"update"},
			RiskLevel:    "medium",
			ActionReason: "",
			AttributeChanges: []report.AttributeChange{
				{Path: "idle_timeout", Before: "60", After: "120"},
				{Path: "tags.Environment", Before: `"staging"`, After: `"dev"`},
			},
		}},
		OutputChanges: []report.OutputChange{{Name: "alb_arn", Actions: []string{"update"}}},
	}, outputFormatTable)
	if err != nil {
		t.Fatalf("write table: %v", err)
	}
	got := output.String()
	for _, want := range []string{
		"Status: changes_detected",
		"MEDIUM  update  aws_lb.main",
		"  idle_timeout: 60 -> 120",
		`  tags.Environment: "staging" -> "dev"`,
		"Output changes:",
		"  alb_arn: update",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, got)
		}
	}
}

func TestWriteScanReportJUnit(t *testing.T) {
	var output bytes.Buffer
	err := writeScanReport(&output, report.DriftReport{Status: report.ScanStatusDriftDetected, TotalChangedResources: 2}, outputFormatJUnit)
	if err != nil {
		t.Fatalf("expected JUnit output to succeed: %v", err)
	}
	for _, want := range []string{`<testsuite name="terradrift" tests="1" failures="1">`, `<failure message="2 resources changed"></failure>`} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("expected JUnit output to contain %q, got %q", want, output.String())
		}
	}
}

func TestWriteScanReportSARIF(t *testing.T) {
	var output bytes.Buffer
	err := writeScanReport(&output, report.DriftReport{ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web"}}}, outputFormatSARIF)
	if err != nil {
		t.Fatalf("expected SARIF output to succeed: %v", err)
	}
	var log sarifLog
	if err := json.Unmarshal(output.Bytes(), &log); err != nil {
		t.Fatalf("expected valid SARIF JSON: %v", err)
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 || len(log.Runs[0].Results) != 1 || log.Runs[0].Results[0].Message.Text != "Terraform drift: aws_instance.web" {
		t.Fatalf("unexpected SARIF log: %#v", log)
	}
}

func TestWriteScanReportSARIFSkipsIgnoredChanges(t *testing.T) {
	var output bytes.Buffer
	err := writeScanReport(&output, report.DriftReport{ResourceChanges: []report.ResourceChange{{Address: "aws_instance.ignored", Ignored: true}, {Address: "aws_instance.active"}}}, outputFormatSARIF)
	if err != nil {
		t.Fatalf("expected SARIF output to succeed: %v", err)
	}
	if strings.Contains(output.String(), "ignored") || !strings.Contains(output.String(), "active") {
		t.Fatalf("expected only active SARIF finding, got %q", output.String())
	}
}

func TestWriteScanReportPrometheus(t *testing.T) {
	var output bytes.Buffer
	err := writeScanReport(&output, report.DriftReport{Status: report.ScanStatusDriftDetected, TotalResourcesChecked: 4, TotalChangedResources: 2}, outputFormatPrometheus)
	if err != nil {
		t.Fatalf("expected Prometheus output to succeed: %v", err)
	}
	for _, want := range []string{"# TYPE terradrift_scan_status gauge", `terradrift_scan_status{status="drift_detected"} 1`, "terradrift_resources_checked 4", "terradrift_resources_changed 2"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("expected Prometheus output to contain %q, got %q", want, output.String())
		}
	}
}

func TestScanAcceptsTimeoutFlag(t *testing.T) {
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--timeout", "1s")
	if err != nil {
		t.Fatalf("expected timeout flag to be accepted, got %v", err)
	}
}

func TestScanUsesConfiguredTerraformBinary(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.tf"), []byte("terraform {}"), 0o600); err != nil {
		t.Fatalf("write Terraform fixture: %v", err)
	}
	_, _, err := executeCommand("scan", "-d", directory, "--terraform-exec", "--terraform-bin", "tofu-not-installed")
	if err == nil || !strings.Contains(err.Error(), "tofu-not-installed") {
		t.Fatalf("expected configured Terraform binary error, got %v", err)
	}
}

func TestScanUsesTerraformBinaryFromConfig(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.tf"), []byte("terraform {}"), 0o600); err != nil {
		t.Fatalf("write Terraform fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	if err := os.WriteFile(path, []byte(`{"directory":"`+filepath.ToSlash(directory)+`","terraform_exec":true,"terraform_bin":"tofu-from-config"}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	_, _, err := executeCommand("scan", "--config", path)
	if err == nil || !strings.Contains(err.Error(), "tofu-from-config") {
		t.Fatalf("expected configured Terraform binary error, got %v", err)
	}
}

func TestInitCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	stdout, _, err := executeCommand("init", "--config", path)
	if err != nil {
		t.Fatalf("expected init to create config: %v", err)
	}
	if !strings.Contains(stdout, "Created TerraDrift config: "+path) {
		t.Fatalf("expected init output to include config path, got %q", stdout)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
	if !strings.Contains(string(data), `"directory": "."`) {
		t.Fatalf("expected default config content, got %q", data)
	}
}

func TestInitCreatesGuidedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	_, _, err := executeCommand("init", "--config", path, "--directory", "terraform/prod", "--terraform-exec", "--redact-paths", "--history-dir", ".history")
	if err != nil {
		t.Fatalf("create guided config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.Directory != "terraform/prod" || !cfg.TerraformExec || !cfg.RedactPaths || cfg.HistoryDir != ".history" {
		t.Fatalf("unexpected guided config: %#v, %v", cfg, err)
	}
}

func TestScanLoadsConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	configJSON := `{
  "directory": "` + filepath.ToSlash(dir) + `",
  "output": "json",
  "timeout": "1s",
  "redact_paths": true
}`
	if err := os.WriteFile(path, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	stdout, _, err := executeCommand("scan", "--config", path)
	if err != nil {
		t.Fatalf("expected scan config to load: %v", err)
	}
	var scanReport report.DriftReport
	if err := json.Unmarshal([]byte(stdout), &scanReport); err != nil {
		t.Fatalf("expected JSON output from config, got %v: %q", err, stdout)
	}
	if scanReport.Directory != "[REDACTED]" {
		t.Fatalf("expected redacted directory from config, got %q", scanReport.Directory)
	}
}

func TestScanLoadsConfigProfile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	configJSON := `{"profiles":{"production":{"directory":"` + filepath.ToSlash(directory) + `","output":"json"}}}`
	if err := os.WriteFile(path, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	stdout, _, err := executeCommand("scan", "--config", path, "--profile", "production")
	if err != nil {
		t.Fatalf("expected profile config to load: %v", err)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("expected JSON output from profile, got %q", stdout)
	}
}

func TestScanLoadsNormalPlanModeFromProfile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	if err := os.WriteFile(path, []byte(`{"profiles":{"production":{"directory":"`+filepath.ToSlash(directory)+`","output":"json","plan_mode":"normal"}}}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	stdout, _, err := executeCommand("scan", "--config", path, "--profile", "production")
	var scanReport report.DriftReport
	if err != nil || json.Unmarshal([]byte(stdout), &scanReport) != nil || scanReport.PlanMode != "normal" || scanReport.Status != report.ScanStatusNoChanges {
		t.Fatalf("unexpected profile report: %q, err=%v", stdout, err)
	}
}

func TestScanLoadsExtendedConfigFile(t *testing.T) {
	dir := t.TempDir()
	dashboardPath := filepath.Join(t.TempDir(), "configured-dashboard.html")
	path := filepath.Join(t.TempDir(), ".terradrift.json")
	configJSON := `{
  "directory": "` + filepath.ToSlash(dir) + `",
  "output": "table",
  "timeout": "1s",
  "redact_paths": true,
  "workspace_root": "` + filepath.ToSlash(dir) + `",
  "dashboard_html": "` + filepath.ToSlash(dashboardPath) + `"
}`
	if err := os.WriteFile(path, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	stdout, _, err := executeCommand("scan", "--config", path)
	if err != nil {
		t.Fatalf("expected extended scan config to load: %v", err)
	}
	if !strings.Contains(stdout, "Terraform directory: [REDACTED]") {
		t.Fatalf("expected config path redaction, got %q", stdout)
	}
	data, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("expected configured dashboard to be written: %v", err)
	}
	if strings.Contains(string(data), dir) {
		t.Fatalf("expected configured dashboard to receive redacted report, got %q", data)
	}
}

func TestScanWritesDashboardHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.html")
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--dashboard-html", path)
	if err != nil {
		t.Fatalf("expected dashboard output to succeed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected dashboard file to exist: %v", err)
	}
	if !strings.Contains(string(data), "TerraDrift Report") {
		t.Fatalf("expected dashboard content, got %q", data)
	}
}

func TestScanWritesHistory(t *testing.T) {
	historyDir := filepath.Join(t.TempDir(), "history")
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--redact-paths", "--history-dir", historyDir)
	if err != nil {
		t.Fatalf("expected history output to succeed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil {
		t.Fatalf("glob history: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one history report, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read history report: %v", err)
	}
	if !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("expected redacted history report, got %q", data)
	}
}

func TestScanDashboardIncludesHistory(t *testing.T) {
	historyDir := filepath.Join(t.TempDir(), "history")
	dashboardPath := filepath.Join(t.TempDir(), "dashboard.html")
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--history-dir", historyDir, "--dashboard-html", dashboardPath)
	if err != nil {
		t.Fatalf("expected dashboard with history to succeed: %v", err)
	}
	data, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	if !strings.Contains(string(data), "Recent scan history") || !strings.Contains(string(data), "no_drift") {
		t.Fatalf("expected dashboard history, got %q", data)
	}
}

func TestScanRunsCostCommand(t *testing.T) {
	stdout, _, err := executeCommand("scan", "-d", t.TempDir(), "--output", "json", "--cost-command", "sh", "--cost-arg", "-c", "--cost-arg", `cat >/dev/null; printf '{"resource_costs":[]}'`)
	if err != nil {
		t.Fatalf("expected cost command to pass: %v", err)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("expected JSON output, got %q", stdout)
	}
}

func TestScanReturnsCostFailure(t *testing.T) {
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--cost-command", "sh", "--cost-arg", "-c", "--cost-arg", "echo password=secret-value >&2; exit 1")
	if err == nil {
		t.Fatal("expected cost failure")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected cost failure to be redacted, got %v", err)
	}
}

func TestScanRunsPolicyCommand(t *testing.T) {
	policyOutput := filepath.Join(t.TempDir(), "policy-input.json")
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--policy-command", "sh", "--policy-arg", "-c", "--policy-arg", "cat > \"$1\"", "--policy-arg", "sh", "--policy-arg", policyOutput)
	if err != nil {
		t.Fatalf("expected policy command to pass: %v", err)
	}
	data, err := os.ReadFile(policyOutput)
	if err != nil {
		t.Fatalf("read policy input: %v", err)
	}
	if !strings.Contains(string(data), `"status":"no_drift"`) {
		t.Fatalf("expected policy input report, got %q", data)
	}
}

func TestScanPolicyFailureSkipsHistory(t *testing.T) {
	historyDir := filepath.Join(t.TempDir(), "history")
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--history-dir", historyDir, "--policy-command", "false")
	if err == nil {
		t.Fatal("expected policy failure")
	}
	entries, err := os.ReadDir(historyDir)
	if err == nil && len(entries) > 0 {
		t.Fatalf("expected no history writes after policy failure, found %d", len(entries))
	}
}

func TestScanAllHelpIncludesDeliveryFlags(t *testing.T) {
	stdout, _, err := executeCommand("scan-all", "--help")
	if err != nil {
		t.Fatalf("scan-all help: %v", err)
	}
	for _, flag := range []string{
		"--history-dir", "--notify", "--policy-command", "--cost-command", "--workspace", "--var-file", "--config",
		"--github-repository", "--github-pr", "--github-issue-after", "--artifact-url", "--approval-file", "--audit-log",
	} {
		if !strings.Contains(stdout, flag) {
			t.Fatalf("expected scan-all help to contain %q", flag)
		}
	}
}

func TestScanHelpIncludesAttributeValuesAndWorkspace(t *testing.T) {
	stdout, _, err := executeCommand("scan", "--help")
	if err != nil {
		t.Fatalf("scan help: %v", err)
	}
	for _, flag := range []string{"--attribute-values", "--workspace", "--var-file", "--var"} {
		if !strings.Contains(stdout, flag) {
			t.Fatalf("expected scan help to contain %q", flag)
		}
	}
}

func TestScanReturnsPolicyFailure(t *testing.T) {
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--policy-command", "sh", "--policy-arg", "-c", "--policy-arg", "echo token=secret-value >&2; exit 1")
	if err == nil {
		t.Fatal("expected policy failure")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected policy failure to be redacted, got %v", err)
	}
}

func TestScanRejectsUnsupportedNotificationTarget(t *testing.T) {
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--notify", "email")
	if err == nil || !strings.Contains(err.Error(), "unsupported notification target") {
		t.Fatalf("expected unsupported notification target error, got %v", err)
	}
}

func TestScanAcceptsCaseInsensitiveTrimmedOutputFormat(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := executeCommand("scan", "-d", dir, "--output", " JSON ")
	if err != nil {
		t.Fatalf("expected normalized output format to be valid, got %v", err)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("expected JSON output, got %q", stdout)
	}
}

func TestScanRejectsUnsupportedOutputFormat(t *testing.T) {
	_, _, err := executeCommand("scan", "--output", "xml")
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("expected unsupported output format error, got %v", err)
	}
}

func TestScanRejectsInvalidPlanMode(t *testing.T) {
	_, _, err := executeCommand("scan", "--plan-mode", "apply")
	if err == nil || !strings.Contains(err.Error(), "unsupported plan mode") {
		t.Fatalf("expected invalid plan mode error, got %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestScanReturnsOutputWriteError(t *testing.T) {
	cmd := newRootCommand(failingWriter{}, &bytes.Buffer{})
	cmd.SetArgs([]string{"scan", "-d", t.TempDir()})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "write scan output") {
		t.Fatalf("expected output write error, got %v", err)
	}
}

func TestExitCodeConstants(t *testing.T) {
	if exitCodeOK != 0 || exitCodeFailure != 1 || exitCodeDriftDetected != 2 {
		t.Fatalf("unexpected exit code constants: ok=%d failure=%d drift=%d", exitCodeOK, exitCodeFailure, exitCodeDriftDetected)
	}
}

func TestExitCodeForDriftDetected(t *testing.T) {
	if got := exitCodeForError(errDriftDetected); got != exitCodeDriftDetected {
		t.Fatalf("expected drift exit code %d, got %d", exitCodeDriftDetected, got)
	}
}

func TestSecretFixturesNeverLeakIntoDeliveryChannels(t *testing.T) {
	secret := "fixture-redaction-probe-v1"
	plan := []byte(`{
		"resource_changes":[{
			"address":"aws_db_instance.main",
			"type":"aws_db_instance",
			"name":"main",
			"mode":"managed",
			"change":{
				"actions":["update"],
				"before":{"password":"` + secret + `","db_conn_str":"` + secret + `","idle_timeout":60},
				"after":{"password":"` + secret + `-2","db_conn_str":"` + secret + `-2","idle_timeout":120}
			}
		}]
	}`)
	changes, _, checked, exact, err := parser.ParsePlan(plan, terraform.PlanModeRefreshOnly)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	scanReport := report.DriftReport{
		ScanID: "test", Status: report.ScanStatusDriftDetected, Directory: "terraform/prod",
		PlanMode: string(terraform.PlanModeRefreshOnly), TotalResourcesChecked: checked,
		ResourcesCheckedExact: exact, TotalChangedResources: len(changes), ResourceChanges: changes,
	}
	encoded, err := json.Marshal(scanReport)
	if err != nil {
		t.Fatalf("marshal live report: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("secret leaked into live JSON: %s", encoded)
	}
	persisted := report.WithoutAttributeValues(scanReport)
	historyDir := t.TempDir()
	if _, err := history.Write(historyDir, persisted); err != nil {
		t.Fatalf("history write: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("history files: %v %#v", err, matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("secret leaked into history: %s", data)
	}
	message := notify.RedactedNotificationMessage(persisted)
	if strings.Contains(message, secret) {
		t.Fatalf("secret leaked into notification: %s", message)
	}
}

func TestScanAllFinalizeWritesHistoryUnderConcurrency(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"development", "production"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("development\nproduction\n"), 0o600); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	historyDir := filepath.Join(t.TempDir(), "history")
	_, _, err := executeCommand(
		"scan-all", "--manifest", manifest, "--output", "json", "--concurrency", "2",
		"--history-dir", historyDir,
		"--policy-command", "true",
	)
	if err != nil {
		t.Fatalf("scan-all: %v", err)
	}
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history files, got %d", len(entries))
	}
}

func TestEnrichAndFinalizeRootAppliesIgnoreAndOwners(t *testing.T) {
	scanReport := report.DriftReport{
		Status:                report.ScanStatusDriftDetected,
		TotalChangedResources: 1,
		ResourceChanges: []report.ResourceChange{
			{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket"},
		},
	}
	params := scanAllParams{
		Enrichment: reportEnrichmentOptions{
			IgnoreRules: []report.IgnoreRule{{
				Address:   "aws_s3_bucket.logs",
				Owner:     "platform",
				Reason:    "temporary exception for test",
				ExpiresAt: time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
			}},
			ResourceOwners: map[string]string{"aws_s3_bucket": "storage-team"},
		},
	}
	if err := enrichAndFinalizeRoot(context.Background(), &scanReport, params); err != nil {
		t.Fatalf("enrichAndFinalizeRoot: %v", err)
	}
	if scanReport.Status != report.ScanStatusNoDrift {
		t.Fatalf("expected ignore to clear drift, got %s", scanReport.Status)
	}
	if !scanReport.ResourceChanges[0].Ignored {
		t.Fatal("expected resource to be ignored")
	}
	if scanReport.ResourceChanges[0].Owner != "storage-team" {
		t.Fatalf("expected type owner, got %q", scanReport.ResourceChanges[0].Owner)
	}
}

func TestScanAllWritesAuditLog(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "development")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("development\n"), 0o600); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	_, _, err := executeCommand(
		"scan-all", "--manifest", manifest, "--output", "json", "--concurrency", "1",
		"--audit-log", auditLog,
	)
	if err != nil {
		t.Fatalf("scan-all: %v", err)
	}
	data, err := os.ReadFile(auditLog)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(data), `"event":"scan_completed"`) {
		t.Fatalf("expected audit event, got %s", data)
	}
}

func TestScanAllRedactsPathsBeforeHistory(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "development")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("development\n"), 0o600); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	historyDir := filepath.Join(t.TempDir(), "history")
	stdout, _, err := executeCommand(
		"scan-all", "--manifest", manifest, "--output", "json", "--concurrency", "1",
		"--redact-paths", "--history-dir", historyDir,
	)
	if err != nil {
		t.Fatalf("scan-all: %v", err)
	}
	if strings.Contains(stdout, root) || strings.Contains(stdout, directory) {
		t.Fatalf("path leaked into stdout: %s", stdout)
	}
	entries, err := os.ReadDir(historyDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("history entries: %v %#v", err, entries)
	}
	data, err := os.ReadFile(filepath.Join(historyDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if strings.Contains(string(data), root) || strings.Contains(string(data), directory) {
		t.Fatalf("path leaked into history: %s", data)
	}
	if !strings.Contains(string(data), `[REDACTED]`) {
		t.Fatalf("expected redacted directory in history, got %s", data)
	}
}

func TestScanAllSurfacesAuditLogWriteErrors(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "development")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("development\n"), 0o600); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	auditLog := t.TempDir() // existing directory; OpenFile for append must fail
	stdout, _, err := executeCommand(
		"scan-all", "--manifest", manifest, "--output", "json", "--concurrency", "1",
		"--audit-log", auditLog,
	)
	if err == nil {
		t.Fatal("expected audit log write failure to fail the multi-root scan")
	}
	var aggregate multiScanReport
	if unmarshalErr := json.Unmarshal([]byte(stdout), &aggregate); unmarshalErr != nil {
		t.Fatalf("aggregate json: %v stdout=%q", unmarshalErr, stdout)
	}
	if aggregate.FailedRoots != 1 || aggregate.Roots[0].Error == "" {
		t.Fatalf("expected failed root from audit write, got %#v", aggregate)
	}
}

func TestAppendScanAllAuditUsesProvidedReportStatus(t *testing.T) {
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	err := appendScanAllAudit(
		scanAllParams{
			Enrichment: reportEnrichmentOptions{AuditLogPath: auditLog},
			Delivery:   deliveryOptions{historyMu: &sync.Mutex{}},
		},
		multiScanRoot{
			Directory: "development",
			Report: report.DriftReport{
				ScanID: "scan-1",
				Status: report.ScanStatusDriftDetected,
			},
		},
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("append audit: %v", err)
	}
	data, err := os.ReadFile(auditLog)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if !strings.Contains(string(data), `"status":"drift_detected"`) {
		t.Fatalf("expected pre-ignore drift status in audit log, got %s", data)
	}
}

func TestScanAllRequiresGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	root := t.TempDir()
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("development\n"), 0o600); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	_, _, err := executeCommand(
		"scan-all", "--manifest", manifest, "--output", "json",
		"--github-repository", "example/terradrift", "--github-pr", "1",
	)
	if err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("expected GITHUB_TOKEN error, got %v", err)
	}
}

func TestScanAllLoadsAllowedCommandsFromConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "development"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := filepath.Join(root, "roots.txt")
	if err := os.WriteFile(manifest, []byte("development\n"), 0o600); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	configPath := filepath.Join(root, ".terradrift.json")
	configBody := `{
  "output": "json",
  "allowed_commands": ["/usr/local/bin/conftest"],
  "policy_command": "true"
}`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("config: %v", err)
	}
	_, _, err := executeCommand("scan-all", "--manifest", manifest, "--config", configPath)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected allowlist rejection for policy command, got %v", err)
	}
}

func TestScanAllHelpIncludesFailureSeverityAndDeliveryParity(t *testing.T) {
	stdout, _, err := executeCommand("scan-all", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, needle := range []string{"--failure-severity", "Delivery matches scan", "ignore/baseline"} {
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected help to contain %q, got %q", needle, stdout)
		}
	}
	for _, stale := range []string{"Delivery subset", "Not yet supported"} {
		if strings.Contains(stdout, stale) {
			t.Fatalf("did not expect outdated help text %q", stale)
		}
	}
}

func TestMultiScanMeetsSeverity(t *testing.T) {
	aggregate := multiScanReport{Roots: []multiScanRoot{
		{Report: report.DriftReport{Status: report.ScanStatusDriftDetected, ResourceChanges: []report.ResourceChange{{RiskLevel: "medium"}}}},
		{Report: report.DriftReport{Status: report.ScanStatusDriftDetected, ResourceChanges: []report.ResourceChange{{RiskLevel: "critical"}}}},
	}}
	meets, err := multiScanMeetsSeverity(aggregate, "high")
	if err != nil || !meets {
		t.Fatalf("expected high threshold to match critical finding: meets=%v err=%v", meets, err)
	}
	meets, err = multiScanMeetsSeverity(aggregate, "critical")
	if err != nil || !meets {
		t.Fatalf("expected critical threshold to match: meets=%v err=%v", meets, err)
	}
	lowOnly := multiScanReport{Roots: []multiScanRoot{
		{Report: report.DriftReport{Status: report.ScanStatusDriftDetected, ResourceChanges: []report.ResourceChange{{RiskLevel: "medium"}}}},
	}}
	meets, err = multiScanMeetsSeverity(lowOnly, "high")
	if err != nil || meets {
		t.Fatalf("expected medium drift below high threshold: meets=%v err=%v", meets, err)
	}
}
