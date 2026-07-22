package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func executeCommand(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
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

func TestScanAcceptsTimeoutFlag(t *testing.T) {
	_, _, err := executeCommand("scan", "-d", t.TempDir(), "--timeout", "1s")
	if err != nil {
		t.Fatalf("expected timeout flag to be accepted, got %v", err)
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

func TestLogLevelSupported(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		_, _, err := executeCommand("--log-level", level, "scan", "-d", t.TempDir())
		if err != nil {
			t.Fatalf("expected log level %q to be supported: %v", level, err)
		}
	}
}

func TestLogLevelConfiguresDefaultLogger(t *testing.T) {
	var stderr bytes.Buffer
	cmd := newRootCommand(&bytes.Buffer{}, &stderr)
	cmd.SetArgs([]string{"--log-level", "debug", "scan", "-d", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected command to execute: %v", err)
	}

	slog.Debug("debug message")
	if !strings.Contains(stderr.String(), "debug message") {
		t.Fatalf("expected default logger to write debug message to stderr, got %q", stderr.String())
	}
}

func TestLogLevelUnsupported(t *testing.T) {
	_, _, err := executeCommand("--log-level", "trace", "scan", "-d", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported log level") {
		t.Fatalf("expected unsupported log level error, got %v", err)
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
