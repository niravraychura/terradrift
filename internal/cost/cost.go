// Package cost enriches drift reports with optional external cost estimates.
package cost

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
	maxCostInputBytes  = 1 << 20
	maxCostOutputBytes = 256 * 1024
)

// Options configures a cost command invocation.
type Options struct {
	Command string
	Args    []string
}

// Validate rejects an incomplete cost command configuration.
func (options Options) Validate() error {
	if strings.TrimSpace(options.Command) == "" {
		return validation.New("cost command", fmt.Errorf("is required"))
	}
	return nil
}

// Enrich runs an external cost command and merges returned cost impacts by resource address.
func Enrich(ctx context.Context, options Options, scanReport report.DriftReport) (report.DriftReport, error) {
	if err := options.Validate(); err != nil {
		return scanReport, err
	}
	command := strings.TrimSpace(options.Command)
	payload, err := json.Marshal(scanReport)
	if err != nil {
		return scanReport, fmt.Errorf("encode cost input: %w", err)
	}
	if len(payload) > maxCostInputBytes {
		return scanReport, fmt.Errorf("cost input exceeds %d bytes", maxCostInputBytes)
	}
	cmd := exec.CommandContext(ctx, command, options.Args...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &limitedBuffer{buffer: &stdout, remaining: maxCostOutputBytes}
	cmd.Stderr = &limitedBuffer{buffer: &stderr, remaining: maxCostOutputBytes}
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return scanReport, fmt.Errorf("cost command failed: %w", err)
		}
		return scanReport, fmt.Errorf("cost command failed: %w: %s", err, redact.String(message))
	}
	var output commandOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return scanReport, fmt.Errorf("parse cost command output: %w", err)
	}
	impacts := make(map[string]string, len(output.ResourceCosts))
	for _, resourceCost := range output.ResourceCosts {
		address := strings.TrimSpace(resourceCost.Address)
		impact := strings.TrimSpace(resourceCost.MonthlyDelta)
		if address != "" && impact != "" {
			impacts[address] = impact
		}
	}
	for index := range scanReport.ResourceChanges {
		if impact := impacts[scanReport.ResourceChanges[index].Address]; impact != "" {
			scanReport.ResourceChanges[index].CostImpact = impact
		}
	}
	return scanReport, nil
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

type commandOutput struct {
	ResourceCosts []resourceCost `json:"resource_costs"`
}

type resourceCost struct {
	Address      string `json:"address"`
	MonthlyDelta string `json:"monthly_delta"`
}
