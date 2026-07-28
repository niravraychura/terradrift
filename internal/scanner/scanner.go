// Package scanner orchestrates TerraDrift scan workflows.
package scanner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/parser"
	"github.com/niravraychura/terradrift/internal/redact"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/niravraychura/terradrift/internal/validation"
)

// DefaultTimeout bounds a scan when no explicit timeout is configured.
const DefaultTimeout = 5 * time.Minute

const scanLockFilename = ".terradrift-scan.lock"

// Outcome describes the automation-relevant result of a scan.
type Outcome string

const (
	// OutcomeNoDrift indicates a completed scan without active drift.
	OutcomeNoDrift Outcome = "no_drift"
	// OutcomeDriftDetected indicates a completed scan with active drift.
	OutcomeDriftDetected Outcome = "drift_detected"
	// OutcomeFailed indicates that scanning could not complete.
	OutcomeFailed Outcome = "failed"
)

// Options configures a scan run.
type Options struct {
	Directory             string
	Timeout               time.Duration
	Runner                terraform.Runner
	WorkspaceRoot         string
	RequireTerraformFiles bool
	workspaceRootResolved bool
}

// Validate rejects invalid scan options before work starts.
func (options Options) Validate() error {
	if options.Timeout < 0 {
		return validation.New("scan timeout", errors.New("must not be negative"))
	}
	return nil
}

// PrepareOptions validates invariant options and resolves the workspace root once.
func PrepareOptions(options Options) (Options, error) {
	if err := options.Validate(); err != nil {
		return Options{}, err
	}
	if options.WorkspaceRoot == "" || options.workspaceRootResolved {
		return options, nil
	}
	root, err := ValidateDirectory(options.WorkspaceRoot)
	if err != nil {
		return Options{}, fmt.Errorf("validate workspace root: %w", err)
	}
	options.WorkspaceRoot = root
	options.workspaceRootResolved = true
	return options, nil
}

// Result captures both the user-facing report and the CLI-facing outcome.
type Result struct {
	Outcome Outcome
	Report  report.DriftReport
}

// Scan validates the requested Terraform directory and optionally runs Terraform.
func Scan(ctx context.Context, options Options) (Result, error) {
	preparedOptions, err := PrepareOptions(options)
	if err != nil {
		return Result{Outcome: OutcomeFailed}, err
	}
	options = preparedOptions
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
		if err := validateResolvedWorkspaceRoot(absDir, options.WorkspaceRoot); err != nil {
			return Result{Outcome: OutcomeFailed}, err
		}
	}
	if options.RequireTerraformFiles {
		matches, err := filepath.Glob(filepath.Join(absDir, "*.tf"))
		if err != nil {
			return Result{Outcome: OutcomeFailed}, fmt.Errorf("list Terraform files: %w", err)
		}
		jsonMatches, err := filepath.Glob(filepath.Join(absDir, "*.tf.json"))
		if err != nil {
			return Result{Outcome: OutcomeFailed}, fmt.Errorf("list Terraform JSON files: %w", err)
		}
		if len(matches)+len(jsonMatches) == 0 {
			return Result{Outcome: OutcomeFailed}, fmt.Errorf("terraform directory has no .tf or .tf.json files: %s", absDir)
		}
	}
	scanID, err := newScanID()
	if err != nil {
		return Result{Outcome: OutcomeFailed}, fmt.Errorf("create scan ID: %w", err)
	}

	if options.Runner == nil {
		now := time.Now().UTC()
		return Result{Outcome: OutcomeNoDrift, Report: report.DriftReport{
			ScanID:          scanID,
			RootID:          rootID(absDir),
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

	scanReport, err := runTerraformScan(ctx, options.Runner, absDir, scanID)
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
	return validateResolvedWorkspaceRoot(resolvedDirectory, resolvedRoot)
}

func validateResolvedWorkspaceRoot(directory string, workspaceRoot string) error {
	rel, err := filepath.Rel(workspaceRoot, directory)
	if err != nil {
		return fmt.Errorf("compare terraform directory to workspace root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("terraform directory %s is outside workspace root %s", directory, workspaceRoot)
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
	resolved, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		return "", fmt.Errorf("resolve terraform directory symlinks: %w", err)
	}
	return resolved, nil
}

func runTerraformScan(ctx context.Context, runner terraform.Runner, directory string, scanID string) (scanReport report.DriftReport, returnErr error) {
	startedAt := time.Now().UTC()
	scanReport = report.DriftReport{
		ScanID:          scanID,
		RootID:          rootID(directory),
		Status:          report.ScanStatusRunning,
		Directory:       directory,
		ResourceChanges: []report.ResourceChange{},
		StartedAt:       startedAt,
	}

	if err := runner.Init(ctx, directory); err != nil {
		failReport(&scanReport, err)
		return scanReport, fmt.Errorf("terraform init: %s", scanReport.ErrorMessage)
	}
	if inventoryRunner, ok := runner.(interface {
		Inventory(context.Context, string) (terraform.Inventory, error)
	}); ok {
		inventory, err := inventoryRunner.Inventory(ctx, directory)
		if err != nil {
			failReport(&scanReport, err)
			return scanReport, fmt.Errorf("terraform inventory: %s", scanReport.ErrorMessage)
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
		failReport(&scanReport, err)
		return scanReport, errors.New(scanReport.ErrorMessage)
	}
	defer func() {
		if err := cleanup(); err != nil && returnErr == nil {
			failReport(&scanReport, err)
			returnErr = fmt.Errorf("remove secure terraform plan file: %s", scanReport.ErrorMessage)
		}
	}()

	exitCode, err := runner.PlanRefreshOnly(ctx, directory, planFile)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, fmt.Errorf("terraform refresh-only plan: %s", scanReport.ErrorMessage)
	}
	if exitCode != 0 && exitCode != 2 {
		err := fmt.Errorf("terraform refresh-only plan failed with exit code %d", exitCode)
		failReport(&scanReport, err)
		return scanReport, err
	}

	planJSON, err := runner.ShowJSON(ctx, directory, planFile)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, fmt.Errorf("terraform show JSON: %s", scanReport.ErrorMessage)
	}

	resourceChanges, outputChanges, totalResources, err := parser.ParsePlan(planJSON)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, errors.New(scanReport.ErrorMessage)
	}

	scanReport.ResourceChanges = resourceChanges
	scanReport.OutputChanges = outputChanges
	scanReport.TotalResourcesChecked = totalResources
	scanReport.TotalChangedResources = len(resourceChanges)
	scanReport.CompletedAt = time.Now().UTC()
	return scanReport, nil
}

func rootID(directory string) string {
	sum := sha256.Sum256([]byte(directory))
	return hex.EncodeToString(sum[:])
}

func newScanID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = bytes[6]&0x0f | 0x40
	bytes[8] = bytes[8]&0x3f | 0x80
	return hex.EncodeToString(bytes[0:4]) + "-" + hex.EncodeToString(bytes[4:6]) + "-" + hex.EncodeToString(bytes[6:8]) + "-" + hex.EncodeToString(bytes[8:10]) + "-" + hex.EncodeToString(bytes[10:]), nil
}

func failReport(scanReport *report.DriftReport, err error) {
	scanReport.Status = report.ScanStatusFailed
	scanReport.CompletedAt = time.Now().UTC()
	scanReport.ErrorMessage = redact.String(err.Error())
}

func securePlanFile(directory string) (string, func() error, error) {
	file, err := os.CreateTemp(directory, ".terradrift-*.tfplan")
	if err != nil {
		return "", func() error { return nil }, fmt.Errorf("create secure terraform plan file: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", func() error { return nil }, fmt.Errorf("secure terraform plan file permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() error { return nil }, fmt.Errorf("close terraform plan file: %w", err)
	}
	return path, func() error {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}, nil
}
