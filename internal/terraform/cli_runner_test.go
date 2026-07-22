package terraform

import (
	"context"
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
