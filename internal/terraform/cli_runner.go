package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/niravraychura/terradrift/internal/redact"
)

const maxCommandOutputBytes = 1 << 20

// CLIRunner executes Terraform-compatible CLI commands.
type CLIRunner struct {
	Path string
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

func (runner CLIRunner) Init(ctx context.Context, directory string) error {
	_, err := runner.run(ctx, directory, "init", "-input=false")
	return err
}

func (runner CLIRunner) PlanRefreshOnly(ctx context.Context, directory string, outputPath string) (int, error) {
	_, err := runner.run(ctx, directory, "plan", "-refresh-only", "-detailed-exitcode", "-out", outputPath)
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}

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
	data, err = os.ReadFile(modulesPath)
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
	inventory.Modules = manifest.Modules
	return inventory, nil
}

func (runner CLIRunner) run(ctx context.Context, directory string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, runner.Path, args...)
	cmd.Dir = directory

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutWriter := &limitedWriter{w: &stdout, n: maxCommandOutputBytes}
	stderrWriter := &limitedWriter{w: &stderr, n: maxCommandOutputBytes}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Run(); err != nil {
		if stdoutWriter.truncated || stderrWriter.truncated {
			return stdout.Bytes(), fmt.Errorf("terraform %v: command output exceeded %d bytes", args, maxCommandOutputBytes)
		}
		if stderr.Len() > 0 {
			return stdout.Bytes(), fmt.Errorf("terraform %v: %w: %s", args, err, redact.String(stderr.String()))
		}
		return stdout.Bytes(), fmt.Errorf("terraform %v: %w", args, err)
	}
	if stdoutWriter.truncated || stderrWriter.truncated {
		return stdout.Bytes(), fmt.Errorf("terraform %v: command output exceeded %d bytes", args, maxCommandOutputBytes)
	}
	return stdout.Bytes(), nil
}

type limitedWriter struct {
	w         io.Writer
	n         int64
	truncated bool
}

func (writer *limitedWriter) Write(p []byte) (int, error) {
	originalLen := len(p)
	if writer.n <= 0 {
		return originalLen, nil
	}
	if int64(len(p)) > writer.n {
		p = p[:writer.n]
		writer.truncated = true
	}
	written, err := writer.w.Write(p)
	writer.n -= int64(written)
	return originalLen, err
}
