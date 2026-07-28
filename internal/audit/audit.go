// Package audit enriches drift reports with external cloud audit events.
package audit

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
	maxInputBytes  = 1 << 20
	maxOutputBytes = 256 * 1024
)

// Options configures an external cloud audit adapter.
type Options struct {
	Command string
	Args    []string
}

// Validate rejects an incomplete audit adapter configuration.
func (options Options) Validate() error {
	if strings.TrimSpace(options.Command) == "" {
		return validation.New("audit command", fmt.Errorf("is required"))
	}
	return nil
}

// Enrich runs an explicit audit adapter and attaches events by Terraform address.
func Enrich(ctx context.Context, options Options, scanReport report.DriftReport) (report.DriftReport, error) {
	if err := options.Validate(); err != nil {
		return scanReport, err
	}
	payload, err := json.Marshal(scanReport)
	if err != nil {
		return scanReport, fmt.Errorf("encode audit input: %w", err)
	}
	if len(payload) > maxInputBytes {
		return scanReport, fmt.Errorf("audit input exceeds %d bytes", maxInputBytes)
	}
	cmd := exec.CommandContext(ctx, options.Command, options.Args...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedBuffer{buffer: &stdout, remaining: maxOutputBytes}
	cmd.Stderr = &limitedBuffer{buffer: &stderr, remaining: maxOutputBytes}
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return scanReport, fmt.Errorf("audit command failed: %w", err)
		}
		return scanReport, fmt.Errorf("audit command failed: %w: %s", err, redact.String(message))
	}
	var output struct {
		ResourceEvents []struct {
			Address string              `json:"address"`
			Events  []report.AuditEvent `json:"events"`
		} `json:"resource_events"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return scanReport, fmt.Errorf("parse audit command output: %w", err)
	}
	events := map[string][]report.AuditEvent{}
	for _, resource := range output.ResourceEvents {
		if strings.TrimSpace(resource.Address) != "" {
			events[resource.Address] = resource.Events
		}
	}
	for i := range scanReport.ResourceChanges {
		scanReport.ResourceChanges[i].AuditEvents = events[scanReport.ResourceChanges[i].Address]
	}
	return scanReport, nil
}

type limitedBuffer struct {
	buffer    *bytes.Buffer
	remaining int
}

func (buffer *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	if buffer.remaining <= 0 {
		return original, nil
	}
	if len(p) > buffer.remaining {
		p = p[:buffer.remaining]
	}
	written, err := buffer.buffer.Write(p)
	buffer.remaining -= written
	return original, err
}
