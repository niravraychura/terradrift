// Package scanner orchestrates TerraDrift scan workflows.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

const DefaultTimeout = 5 * time.Minute

// Outcome describes the automation-relevant result of a scan.
type Outcome string

const (
	OutcomeNoDrift       Outcome = "no_drift"
	OutcomeDriftDetected Outcome = "drift_detected"
	OutcomeFailed        Outcome = "failed"
)

// Options configures a scan run.
type Options struct {
	Directory string
	Timeout   time.Duration
}

// Result captures both the user-facing report and the CLI-facing outcome.
type Result struct {
	Outcome Outcome
	Report  report.DriftReport
}

// Scan validates the requested Terraform directory and returns a bootstrap no-drift result.
func Scan(ctx context.Context, options Options) (Result, error) {
	if options.Timeout <= 0 {
		options.Timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return Result{Outcome: OutcomeFailed}, fmt.Errorf("scan timed out before starting: %w", ctx.Err())
	default:
	}

	absDir, err := ValidateDirectory(options.Directory)
	if err != nil {
		return Result{Outcome: OutcomeFailed}, err
	}

	scanReport := NewBootstrapReport(absDir)
	return Result{Outcome: OutcomeNoDrift, Report: scanReport}, nil
}

// ValidateDirectory resolves and validates the local directory selected for scanning.
func ValidateDirectory(directory string) (string, error) {
	if directory == "" {
		directory = "."
	}
	absDir, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve terraform directory: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("terraform directory does not exist: %s", absDir)
		}
		return "", fmt.Errorf("inspect terraform directory %s: %w", absDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("terraform path is not a directory: %s", absDir)
	}
	return absDir, nil
}

// NewBootstrapReport creates the placeholder report used until Terraform execution is implemented.
func NewBootstrapReport(directory string) report.DriftReport {
	now := time.Now().UTC()
	return report.DriftReport{
		Status:          report.ScanStatusNoDrift,
		Directory:       directory,
		ResourceChanges: []report.ResourceChange{},
		StartedAt:       now,
		CompletedAt:     now,
	}
}
