package terraform

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIRunnerShowJSONCapturesOutput(t *testing.T) {
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
if [ "$1" = "show" ]; then
  printf '{"resource_changes":[]}'
fi
`))

	output, err := runner.ShowJSON(context.Background(), t.TempDir(), "plan.tfplan")
	if err != nil {
		t.Fatalf("expected show to succeed: %v", err)
	}
	if string(output) != `{"resource_changes":[]}` {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestCLIRunnerPlanRefreshOnlyReturnsDetailedExitCode(t *testing.T) {
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
exit 2
`))

	exitCode, err := runner.PlanRefreshOnly(context.Background(), t.TempDir(), "plan.tfplan")
	if err != nil {
		t.Fatalf("expected detailed exit code without command error, got %v", err)
	}
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
}

func TestCLIRunnerRedactsStderr(t *testing.T) {
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
printf 'token=super-secret' >&2
exit 1
`))

	err := runner.Init(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected command error")
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("expected stderr secret to be redacted, got %v", err)
	}
}

func TestLimitedWriterMarksTruncation(t *testing.T) {
	writer := &limitedWriter{w: io.Discard, n: 1}
	if _, err := writer.Write([]byte("ab")); err != nil || !writer.truncated {
		t.Fatalf("expected truncation, err=%v", err)
	}
}

func TestCLIRunnerInventory(t *testing.T) {
	directory := t.TempDir()
	modulesDir := filepath.Join(directory, ".terraform", "modules")
	if err := os.MkdirAll(modulesDir, 0o700); err != nil {
		t.Fatalf("create modules fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulesDir, "modules.json"), []byte(`{"Modules":[{"Key":"network","Source":"hashicorp/network/aws","Version":"1.0.0"}]}`), 0o600); err != nil {
		t.Fatalf("write modules fixture: %v", err)
	}
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
if [ "$1" = "version" ]; then
  printf '{"terraform_version":"1.10.0","provider_selections":{"registry.terraform.io/hashicorp/aws":"5.0.0"}}'
fi
`))

	inventory, err := runner.Inventory(context.Background(), directory)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if inventory.TerraformVersion != "1.10.0" || inventory.ProviderVersions["registry.terraform.io/hashicorp/aws"] != "5.0.0" || len(inventory.Modules) != 1 || inventory.Modules[0].Source != "hashicorp/network/aws" {
		t.Fatalf("unexpected inventory: %#v", inventory)
	}
}

func writeTerraformStub(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub requires POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "terraform")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write terraform stub: %v", err)
	}
	return path
}
