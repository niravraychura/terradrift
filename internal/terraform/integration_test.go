package terraform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCLIRunnerIntegration(t *testing.T) {
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform is not installed")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.tf"), []byte("terraform {\n  required_version = \">= 1.0.0\"\n}\n"), 0o600); err != nil {
		t.Fatalf("write Terraform fixture: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runner := NewCLIRunner(terraformPath)
	if err := runner.Init(ctx, directory); err != nil {
		t.Fatalf("terraform init: %v", err)
	}
	planPath := filepath.Join(directory, "plan.tfplan")
	exitCode, err := runner.Plan(ctx, directory, planPath, PlanModeRefreshOnly)
	if err != nil || exitCode != 0 {
		t.Fatalf("terraform plan: exit=%d err=%v", exitCode, err)
	}
	if _, err := runner.ShowJSON(ctx, directory, planPath); err != nil {
		t.Fatalf("terraform show: %v", err)
	}
}
