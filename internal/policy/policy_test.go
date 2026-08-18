package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestRunPassesReportJSONOnStdin(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "policy-input.json")
	err := Run(context.Background(), Options{Command: "sh", Args: []string{"-c", "cat > \"$1\"", "sh", outputPath}}, report.DriftReport{Status: report.ScanStatusNoDrift})
	if err != nil {
		t.Fatalf("expected policy command to pass: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read policy input: %v", err)
	}
	if !strings.Contains(string(data), `"status":"no_drift"`) {
		t.Fatalf("expected scan report JSON on stdin, got %q", data)
	}
}

func TestRunRedactsPolicyFailures(t *testing.T) {
	err := Run(context.Background(), Options{Command: "sh", Args: []string{"-c", "echo 'token=secret-value' >&2; exit 1"}}, report.DriftReport{})
	if err == nil {
		t.Fatal("expected policy failure")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected policy error to be redacted, got %v", err)
	}
}

func TestRunRejectsOversizedInput(t *testing.T) {
	err := Run(context.Background(), Options{Command: "false"}, report.DriftReport{ErrorMessage: strings.Repeat("x", maxPolicyInputBytes)})
	if err == nil || !strings.Contains(err.Error(), "policy input exceeds") {
		t.Fatalf("expected oversized input error, got %v", err)
	}
}

func TestRunFailsClosedOnTruncatedOutput(t *testing.T) {
	previous := maxPolicyOutputBytes
	maxPolicyOutputBytes = 8
	t.Cleanup(func() { maxPolicyOutputBytes = previous })
	err := Run(context.Background(), Options{Command: "sh", Args: []string{"-c", "printf '%s' '0123456789abcdef'"}}, report.DriftReport{})
	if err == nil || !strings.Contains(err.Error(), "command output exceeded") {
		t.Fatalf("expected truncation error, got %v", err)
	}
}
