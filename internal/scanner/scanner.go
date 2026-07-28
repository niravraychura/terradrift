// Package scanner orchestrates TerraDrift scan workflows.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/parser"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/terraform"
)

const DefaultTimeout = 5 * time.Minute

const scanLockFilename = ".terradrift-scan.lock"

// Outcome describes the automation-relevant result of a scan.
type Outcome string

const (
	OutcomeNoDrift       Outcome = "no_drift"
	OutcomeDriftDetected Outcome = "drift_detected"
	OutcomeFailed        Outcome = "failed"
)

// Options configures a scan run.
type Options struct {
	Directory     string
	Timeout       time.Duration
	Runner        terraform.Runner
	WorkspaceRoot string
}

// Result captures both the user-facing report and the CLI-facing outcome.
type Result struct {
	Outcome Outcome
	Report  report.DriftReport
}

// Scan validates the requested Terraform directory and optionally runs Terraform.
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
	if options.WorkspaceRoot != "" {
		if err := ValidateWorkspaceRoot(absDir, options.WorkspaceRoot); err != nil {
			return Result{Outcome: OutcomeFailed}, err
		}
	}

	if options.Runner == nil {
		now := time.Now().UTC()
		return Result{Outcome: OutcomeNoDrift, Report: report.DriftReport{
			Status:          report.ScanStatusNoDrift,
			Directory:       absDir,
			ResourceChanges: []report.ResourceChange{},
			StartedAt:       now,
			CompletedAt:     now,
		}}, nil
	}

	unlock, err := acquireScanLock(absDir)
	if err != nil {
		return Result{Outcome: OutcomeFailed}, err
	}
	defer unlock()

	scanReport, err := runTerraformScan(ctx, options.Runner, absDir)
	if err != nil {
		return Result{Outcome: OutcomeFailed, Report: scanReport}, err
	}
	if scanReport.TotalChangedResources > 0 {
		scanReport.Status = report.ScanStatusDriftDetected
		return Result{Outcome: OutcomeDriftDetected, Report: scanReport}, nil
	}
	scanReport.Status = report.ScanStatusNoDrift
	return Result{Outcome: OutcomeNoDrift, Report: scanReport}, nil
}

func acquireScanLock(directory string) (func(), error) {
	path := filepath.Join(directory, scanLockFilename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("terraform scan already running for %s; remove stale %s after confirming no scan is active", directory, path)
		}
		return nil, fmt.Errorf("create terraform scan lock: %w", err)
	}
	if _, err := fmt.Fprintln(file, os.Getpid()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write terraform scan lock: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close terraform scan lock: %w", err)
	}
	// ponytail: local O_EXCL lock; use a shared lock service for distributed runners.
	return func() { _ = os.Remove(path) }, nil
}

// ValidateWorkspaceRoot ensures directory resolves inside workspaceRoot after symlink evaluation.
func ValidateWorkspaceRoot(directory string, workspaceRoot string) error {
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("resolve terraform directory symlinks: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace root symlinks: %w", err)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedDirectory)
	if err != nil {
		return fmt.Errorf("compare terraform directory to workspace root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("terraform directory %s is outside workspace root %s", resolvedDirectory, resolvedRoot)
	}
	return nil
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

func runTerraformScan(ctx context.Context, runner terraform.Runner, directory string) (report.DriftReport, error) {
	startedAt := time.Now().UTC()
	scanReport := report.DriftReport{
		Status:          report.ScanStatusRunning,
		Directory:       directory,
		ResourceChanges: []report.ResourceChange{},
		StartedAt:       startedAt,
	}

	if err := runner.Init(ctx, directory); err != nil {
		scanReport.Status = report.ScanStatusFailed
		scanReport.CompletedAt = time.Now().UTC()
		scanReport.ErrorMessage = err.Error()
		return scanReport, fmt.Errorf("terraform init: %w", err)
	}
	if inventoryRunner, ok := runner.(interface {
		Inventory(context.Context, string) (terraform.Inventory, error)
	}); ok {
		inventory, err := inventoryRunner.Inventory(ctx, directory)
		if err != nil {
			scanReport.Status = report.ScanStatusFailed
			scanReport.CompletedAt = time.Now().UTC()
			scanReport.ErrorMessage = err.Error()
			return scanReport, fmt.Errorf("terraform inventory: %w", err)
		}
		scanReport.TerraformVersion = inventory.TerraformVersion
		scanReport.ProviderVersions = inventory.ProviderVersions
		scanReport.Modules = make([]report.ModuleInventory, len(inventory.Modules))
		for i, module := range inventory.Modules {
			scanReport.Modules[i] = report.ModuleInventory{Key: module.Key, Source: module.Source, Version: module.Version}
		}
	}

	planFile, cleanup, err := securePlanFile(directory)
	if err != nil {
		scanReport.Status = report.ScanStatusFailed
		scanReport.CompletedAt = time.Now().UTC()
		scanReport.ErrorMessage = err.Error()
		return scanReport, err
	}
	defer cleanup()

	exitCode, err := runner.PlanRefreshOnly(ctx, directory, planFile)
	if err != nil {
		scanReport.Status = report.ScanStatusFailed
		scanReport.CompletedAt = time.Now().UTC()
		scanReport.ErrorMessage = err.Error()
		return scanReport, fmt.Errorf("terraform refresh-only plan: %w", err)
	}
	if exitCode != 0 && exitCode != 2 {
		err := fmt.Errorf("terraform refresh-only plan failed with exit code %d", exitCode)
		scanReport.Status = report.ScanStatusFailed
		scanReport.CompletedAt = time.Now().UTC()
		scanReport.ErrorMessage = err.Error()
		return scanReport, err
	}

	planJSON, err := runner.ShowJSON(ctx, directory, planFile)
	if err != nil {
		scanReport.Status = report.ScanStatusFailed
		scanReport.CompletedAt = time.Now().UTC()
		scanReport.ErrorMessage = err.Error()
		return scanReport, fmt.Errorf("terraform show JSON: %w", err)
	}

	resourceChanges, totalResources, err := parser.ParsePlan(planJSON)
	if err != nil {
		scanReport.Status = report.ScanStatusFailed
		scanReport.CompletedAt = time.Now().UTC()
		scanReport.ErrorMessage = err.Error()
		return scanReport, err
	}

	scanReport.ResourceChanges = resourceChanges
	scanReport.TotalResourcesChecked = totalResources
	scanReport.TotalChangedResources = len(resourceChanges)
	scanReport.CompletedAt = time.Now().UTC()
	return scanReport, nil
}

func securePlanFile(directory string) (string, func(), error) {
	file, err := os.CreateTemp(directory, ".terradrift-*.tfplan")
	if err != nil {
		return "", func() {}, fmt.Errorf("create secure terraform plan file: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("secure terraform plan file permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("close terraform plan file: %w", err)
	}
	return path, func() { _ = os.Remove(path) }, nil
}
