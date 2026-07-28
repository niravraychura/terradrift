// Package policy runs optional policy-as-code checks against scan reports.
package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/niravraychura/terradrift/internal/redact"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/validation"
)

const (
	maxPolicyInputBytes  = 1 << 20
	maxPolicyOutputBytes = 64 * 1024
)

// Options configures a policy command invocation.
type Options struct {
	Command string
	Args    []string
}

// Validate rejects an incomplete policy command configuration.
func (options Options) Validate() error {
	if strings.TrimSpace(options.Command) == "" {
		return validation.New("policy command", fmt.Errorf("is required"))
	}
	return nil
}

// Run executes a policy command with the scan report JSON on stdin.
func Run(ctx context.Context, options Options, scanReport report.DriftReport) error {
	if err := options.Validate(); err != nil {
		return err
	}
	command := strings.TrimSpace(options.Command)
	payload, err := json.Marshal(scanReport)
	if err != nil {
		return fmt.Errorf("encode policy input: %w", err)
	}
	if len(payload) > maxPolicyInputBytes {
		return fmt.Errorf("policy input exceeds %d bytes", maxPolicyInputBytes)
	}
	cmd := exec.CommandContext(ctx, command, options.Args...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &limitedBuffer{buffer: &stdout, remaining: maxPolicyOutputBytes}
	cmd.Stderr = &limitedBuffer{buffer: &stderr, remaining: maxPolicyOutputBytes}
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return fmt.Errorf("policy command failed: %w", err)
		}
		return fmt.Errorf("policy command failed: %w: %s", err, redact.String(message))
	}
	return nil
}

type limitedBuffer struct {
	buffer    *bytes.Buffer
	remaining int
}

func (buffer *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	if buffer.remaining <= 0 {
		return originalLen, nil
	}
	if len(p) > buffer.remaining {
		p = p[:buffer.remaining]
	}
	written, err := buffer.buffer.Write(p)
	buffer.remaining -= written
	return originalLen, err
}
