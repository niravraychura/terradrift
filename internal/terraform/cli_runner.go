package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/redact"
)

const (
	maxCommandOutputBytes   = 32 << 20
	maxModulesManifestBytes = 32 << 20
	// DefaultLockTimeout is how long terraform plan waits for the remote state lock.
	DefaultLockTimeout = 10 * time.Minute
)

// CLIRunner executes Terraform-compatible CLI commands.
type CLIRunner struct {
	Path        string
	Workspace   string
	VarFiles    []string
	Vars        []string
	DisableLock bool
	LockTimeout time.Duration
}

// Inventory describes the selected CLI, providers, and initialized modules.
type Inventory struct {
	TerraformVersion string
	ProviderVersions map[string]string
	Modules          []Module
}

// Module identifies an initialized module without exposing its local directory.
type Module struct {
	Key     string `json:"Key"`
	Source  string `json:"Source"`
	Version string `json:"Version"`
}

// NewCLIRunner creates a Terraform-compatible CLI runner. Empty path defaults to terraform.
func NewCLIRunner(path string) CLIRunner {
	if path == "" {
		path = "terraform"
	}
	return CLIRunner{Path: path}
}

// Init initializes Terraform without upgrades or lockfile writes.
func (runner CLIRunner) Init(ctx context.Context, directory string) error {
	// Drift scans must neither upgrade providers nor modify the dependency lock file.
	_, err := runner.run(ctx, directory, "init", "-input=false", "-backend=true", "-lockfile=readonly")
	return err
}

// Plan runs Terraform's selected plan mode and returns its detailed exit code.
func (runner CLIRunner) Plan(ctx context.Context, directory string, outputPath string, mode PlanMode) (int, error) {
	mode, err := ParsePlanMode(string(mode))
	if err != nil {
		return 1, err
	}
	if err := runner.selectWorkspace(ctx, directory); err != nil {
		return 1, err
	}
	args := []string{"plan", "-input=false"}
	if mode == PlanModeRefreshOnly {
		args = append(args, "-refresh-only")
	}
	args = append(args, "-detailed-exitcode", "-out", outputPath)
	if runner.DisableLock {
		args = append(args, "-lock=false")
	} else {
		timeout := runner.LockTimeout
		if timeout <= 0 {
			timeout = DefaultLockTimeout
		}
		args = append(args, "-lock=true", "-lock-timeout="+timeout.String())
	}
	for _, varFile := range runner.VarFiles {
		args = append(args, "-var-file="+varFile)
	}
	for _, variable := range runner.Vars {
		args = append(args, "-var="+variable)
	}
	_, err = runner.run(ctx, directory, args...)
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}

func (runner CLIRunner) selectWorkspace(ctx context.Context, directory string) error {
	workspace := strings.TrimSpace(runner.Workspace)
	if workspace == "" {
		return nil
	}
	_, err := runner.run(ctx, directory, "workspace", "select", "-input=false", workspace)
	if err != nil {
		return fmt.Errorf("terraform workspace select %q: %w", workspace, err)
	}
	return nil
}

// ShowJSON returns the JSON rendering of a Terraform plan file.
func (runner CLIRunner) ShowJSON(ctx context.Context, directory string, planPath string) ([]byte, error) {
	return runner.run(ctx, directory, "show", "-json", planPath)
}

// Inventory returns CLI version data and the initialized module manifest.
func (runner CLIRunner) Inventory(ctx context.Context, directory string) (Inventory, error) {
	data, err := runner.run(ctx, directory, "version", "-json")
	if err != nil {
		return Inventory{}, err
	}
	var version struct {
		TerraformVersion   string            `json:"terraform_version"`
		ProviderSelections map[string]string `json:"provider_selections"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return Inventory{}, fmt.Errorf("parse Terraform version JSON: %w", err)
	}
	inventory := Inventory{TerraformVersion: version.TerraformVersion, ProviderVersions: version.ProviderSelections, Modules: []Module{}}
	modulesPath := filepath.Join(directory, ".terraform", "modules", "modules.json")
	data, err = ioutil.ReadLimitedFile(modulesPath, maxModulesManifestBytes)
	if os.IsNotExist(err) {
		return inventory, nil
	}
	if err != nil {
		return Inventory{}, fmt.Errorf("read Terraform modules manifest: %w", err)
	}
	var manifest struct {
		Modules []Module `json:"Modules"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Inventory{}, fmt.Errorf("parse Terraform modules manifest: %w", err)
	}
	for i := range manifest.Modules {
		manifest.Modules[i].Source = redactModuleSource(manifest.Modules[i].Source)
	}
	inventory.Modules = manifest.Modules
	return inventory, nil
}

func redactModuleSource(source string) string {
	if prefix, urlSource, found := strings.Cut(source, "::"); found {
		return prefix + "::" + redact.String(urlSource)
	}
	return redact.String(source)
}

func (runner CLIRunner) run(ctx context.Context, directory string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, runner.Path, withNoColor(args)...)
	cmd.Dir = directory
	cmd.Env = withPlannerAutomation(os.Environ(), runner.Path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutWriter := &ioutil.LimitedWriter{W: &stdout, Remaining: maxCommandOutputBytes}
	stderrWriter := &ioutil.LimitedWriter{W: &stderr, Remaining: maxCommandOutputBytes}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Run(); err != nil {
		if stdoutWriter.Truncated || stderrWriter.Truncated {
			return stdout.Bytes(), fmt.Errorf("terraform %v: command output exceeded %d bytes", args, maxCommandOutputBytes)
		}
		if stderr.Len() > 0 {
			return stdout.Bytes(), fmt.Errorf("terraform %v: %w: %s", args, err, redact.String(stderr.String()))
		}
		return stdout.Bytes(), fmt.Errorf("terraform %v: %w", args, err)
	}
	if stdoutWriter.Truncated || stderrWriter.Truncated {
		return stdout.Bytes(), fmt.Errorf("terraform %v: command output exceeded %d bytes", args, maxCommandOutputBytes)
	}
	return stdout.Bytes(), nil
}

// withNoColor inserts -no-color after the Terraform subcommand so wrappers that
// require argv[1] to be init/plan/show (hashicorp/setup-terraform) keep working.
func withNoColor(args []string) []string {
	if len(args) == 0 {
		return []string{"-no-color"}
	}
	out := make([]string, 0, len(args)+1)
	out = append(out, args[0], "-no-color")
	return append(out, args[1:]...)
}

func withPlannerAutomation(environ []string, binPath string) []string {
	out := withTerraformAutomation(environ)
	if !isTerragruntBinary(binPath) {
		return out
	}
	filtered := make([]string, 0, len(out)+2)
	for _, entry := range out {
		if strings.HasPrefix(entry, "TG_NON_INTERACTIVE=") || strings.HasPrefix(entry, "TERRAGRUNT_NON_INTERACTIVE=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "TG_NON_INTERACTIVE=true", "TERRAGRUNT_NON_INTERACTIVE=true")
}

func withTerraformAutomation(environ []string) []string {
	out := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if strings.HasPrefix(entry, "TF_IN_AUTOMATION=") {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "TF_IN_AUTOMATION=1")
}
