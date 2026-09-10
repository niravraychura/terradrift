package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/spf13/cobra"
)

func terraformExecRequired() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")), "true") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TERRADRIFT_REQUIRE_EXEC"))) {
	case "", "0", "false", "no":
		return false
	default:
		return true
	}
}

func errTerraformExecRequired() error {
	return errors.New("terraform execution is required in CI; pass --terraform-exec (or set terraform_exec in config)")
}

func rejectDeadGitHubNotify(notifyTarget string) error {
	if strings.EqualFold(strings.TrimSpace(notifyTarget), "github") {
		return errors.New("--notify github is not supported; use --github-pr or --github-issue-after")
	}
	return nil
}

func configureCLIRunner(runner terraform.CLIRunner, workspace string, varFiles, vars []string, stateLock bool, lockTimeout time.Duration) terraform.CLIRunner {
	runner.Workspace = workspace
	runner.VarFiles = append([]string(nil), varFiles...)
	runner.Vars = append([]string(nil), vars...)
	runner.DisableLock = !stateLock
	runner.LockTimeout = lockTimeout
	return runner
}

func warnStateLockDisabled(cmd *cobra.Command, stateLock bool) {
	if stateLock {
		return
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: --state-lock=false skips Terraform's remote state lock; concurrent apply can race")
}
