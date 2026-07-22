package terraform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/niravraychura/terradrift/internal/redact"
)

const maxCommandOutputBytes = 1 << 20

// CLIRunner executes Terraform commands through the terraform binary.
type CLIRunner struct {
	Path string
}

// NewCLIRunner creates a Terraform CLI runner. Empty path defaults to terraform.
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

func (runner CLIRunner) run(ctx context.Context, directory string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, runner.Path, args...)
	cmd.Dir = directory

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, n: maxCommandOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, n: maxCommandOutputBytes}

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return stdout.Bytes(), fmt.Errorf("terraform %v: %w: %s", args, err, redact.String(stderr.String()))
		}
		return stdout.Bytes(), fmt.Errorf("terraform %v: %w", args, err)
	}
	return stdout.Bytes(), nil
}

type limitedWriter struct {
	w io.Writer
	n int64
}

func (writer *limitedWriter) Write(p []byte) (int, error) {
	originalLen := len(p)
	if writer.n <= 0 {
		return originalLen, nil
	}
	if int64(len(p)) > writer.n {
		p = p[:writer.n]
	}
	written, err := writer.w.Write(p)
	writer.n -= int64(written)
	return originalLen, err
}
