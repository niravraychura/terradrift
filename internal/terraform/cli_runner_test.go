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

func TestCLIRunnerPlanReturnsDetailedExitCode(t *testing.T) {
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
exit 2
`))

	exitCode, err := runner.Plan(context.Background(), t.TempDir(), "plan.tfplan", PlanModeRefreshOnly)
	if err != nil {
		t.Fatalf("expected detailed exit code without command error, got %v", err)
	}
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
}

func TestCLIRunnerPlanUsesModeArguments(t *testing.T) {
	for _, test := range []struct {
		mode PlanMode
		want string
	}{
		{PlanModeRefreshOnly, "plan -refresh-only -detailed-exitcode -out plan.tfplan"},
		{PlanModeNormal, "plan -detailed-exitcode -out plan.tfplan"},
	} {
		t.Run(string(test.mode), func(t *testing.T) {
			runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
printf '%s' "$*" > "$TERRADRIFT_ARGS"
`))
			argsPath := filepath.Join(t.TempDir(), "args")
			t.Setenv("TERRADRIFT_ARGS", argsPath)
			if _, err := runner.Plan(context.Background(), t.TempDir(), "plan.tfplan", test.mode); err != nil {
				t.Fatalf("plan: %v", err)
			}
			data, err := os.ReadFile(argsPath)
			if err != nil || string(data) != test.want {
				t.Fatalf("arguments = %q, err=%v, want %q", data, err, test.want)
			}
		})
	}
}

func TestCLIRunnerPlanSelectsWorkspaceAndPassesVars(t *testing.T) {
	commandsPath := filepath.Join(t.TempDir(), "commands")
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
printf '%s\n' "$*" >> "$TERRADRIFT_COMMANDS"
`))
	runner.Workspace = "staging"
	runner.VarFiles = []string{"prod.tfvars"}
	runner.Vars = []string{"region=us-east-1"}
	t.Setenv("TERRADRIFT_COMMANDS", commandsPath)
	if _, err := runner.Plan(context.Background(), t.TempDir(), "plan.tfplan", PlanModeNormal); err != nil {
		t.Fatalf("plan: %v", err)
	}
	data, err := os.ReadFile(commandsPath)
	if err != nil {
		t.Fatalf("read commands: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected workspace select then plan, got %q", data)
	}
	if lines[0] != "workspace select staging" {
		t.Fatalf("workspace command = %q", lines[0])
	}
	if !strings.Contains(lines[1], "-var-file=prod.tfvars") || !strings.Contains(lines[1], "-var=region=us-east-1") {
		t.Fatalf("plan args = %q", lines[1])
	}
}

func TestCLIRunnerWorkspaceSelectFailure(t *testing.T) {
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
if [ "$1" = "workspace" ]; then
  printf 'no such workspace' >&2
  exit 1
fi
`))
	runner.Workspace = "missing"
	_, err := runner.Plan(context.Background(), t.TempDir(), "plan.tfplan", PlanModeNormal)
	if err == nil || !strings.Contains(err.Error(), `workspace select "missing"`) {
		t.Fatalf("expected workspace select error, got %v", err)
	}
}

func TestParsePlanModeRejectsInvalidValue(t *testing.T) {
	if _, err := ParsePlanMode("apply"); err == nil {
		t.Fatal("expected invalid mode error")
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

func TestCLIRunnerInitUsesSafeFlags(t *testing.T) {
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
printf '%s' "$*" > "$TERRADRIFT_ARGS"
`))
	argsPath := filepath.Join(t.TempDir(), "args")
	t.Setenv("TERRADRIFT_ARGS", argsPath)
	if err := runner.Init(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("init: %v", err)
	}
	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read arguments: %v", err)
	}
	if string(data) != "init -input=false -backend=true -lockfile=readonly" {
		t.Fatalf("unexpected init arguments: %q", data)
	}
}

func TestLimitedWriterMarksTruncation(t *testing.T) {
	writer := &limitedWriter{w: io.Discard, n: 1}
	if _, err := writer.Write([]byte("ab")); err != nil || !writer.truncated {
		t.Fatalf("expected truncation, err=%v", err)
	}
}

func TestLimitedWriterMarksTruncationAfterExactLimit(t *testing.T) {
	writer := &limitedWriter{w: io.Discard, n: 1}
	if _, err := writer.Write([]byte("a")); err != nil || writer.truncated {
		t.Fatalf("expected exact limit to succeed, err=%v truncated=%t", err, writer.truncated)
	}
	if _, err := writer.Write([]byte("b")); err != nil || !writer.truncated {
		t.Fatalf("expected subsequent output to mark truncation, err=%v truncated=%t", err, writer.truncated)
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

func TestCLIRunnerInventoryRedactsModuleSource(t *testing.T) {
	directory := t.TempDir()
	modulesDir := filepath.Join(directory, ".terraform", "modules")
	if err := os.MkdirAll(modulesDir, 0o700); err != nil {
		t.Fatalf("create modules fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulesDir, "modules.json"), []byte(`{"Modules":[{"Source":"git::https://token@modules.example.com/network"}]}`), 0o600); err != nil {
		t.Fatalf("write modules fixture: %v", err)
	}
	runner := NewCLIRunner(writeTerraformStub(t, `#!/bin/sh
printf '{"terraform_version":"1.10.0"}'
`))
	inventory, err := runner.Inventory(context.Background(), directory)
	if err != nil || strings.Contains(inventory.Modules[0].Source, "token") {
		t.Fatalf("expected redacted module source, inventory=%#v err=%v", inventory, err)
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
