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

	"github.com/niravraychura/terradrift/internal/logger"
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
	// OutcomeNoChanges indicates a completed normal plan without changes.
	OutcomeNoChanges Outcome = "no_changes"
	// OutcomeChangesDetected indicates a completed normal plan with changes.
	OutcomeChangesDetected Outcome = "changes_detected"
	// OutcomeFailed indicates that scanning could not complete.
	OutcomeFailed Outcome = "failed"
)

// Options configures a scan run.
type Options struct {
	Directory             string
	Timeout               time.Duration
	Runner                terraform.Runner
	PlanMode              terraform.PlanMode
	WorkspaceRoot         string
	RequireTerraformFiles bool
	LockBackend           LockBackend
	SkipInit              bool
	workspaceRootResolved bool
}

// Validate rejects invalid scan options before work starts.
func (options Options) Validate() error {
	if options.Timeout < 0 {
		return validation.New("scan timeout", errors.New("must not be negative"))
	}
	if _, err := terraform.ParsePlanMode(string(options.PlanMode)); err != nil {
		return validation.New("scan plan mode", err)
	}
	return nil
}

// PrepareOptions validates invariant options and resolves the workspace root once.
func PrepareOptions(options Options) (Options, error) {
	if err := options.Validate(); err != nil {
		return Options{}, err
	}
	if options.WorkspaceRoot == "" || options.workspaceRootResolved {
		mode, err := terraform.ParsePlanMode(string(options.PlanMode))
		if err != nil {
			return Options{}, validation.New("scan plan mode", err)
		}
		options.PlanMode = mode
		return options, nil
	}
	root, err := ValidateDirectory(options.WorkspaceRoot)
	if err != nil {
		return Options{}, fmt.Errorf("validate workspace root: %w", err)
	}
	options.WorkspaceRoot = root
	options.workspaceRootResolved = true
	mode, err := terraform.ParsePlanMode(string(options.PlanMode))
	if err != nil {
		return Options{}, validation.New("scan plan mode", err)
	}
	options.PlanMode = mode
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
		logger.Error(ctx, "scan failed", "directory", options.Directory, "error", err)
		return Result{Outcome: OutcomeFailed}, err
	}
	logger.Info(ctx, "scan started", "directory", absDir)
	if options.WorkspaceRoot != "" {
		if err := validateResolvedWorkspaceRoot(absDir, options.WorkspaceRoot); err != nil {
			logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
			return Result{Outcome: OutcomeFailed}, err
		}
	}
	if options.RequireTerraformFiles {
		matches, err := filepath.Glob(filepath.Join(absDir, "*.tf"))
		if err != nil {
			logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
			return Result{Outcome: OutcomeFailed}, fmt.Errorf("list Terraform files: %w", err)
		}
		jsonMatches, err := filepath.Glob(filepath.Join(absDir, "*.tf.json"))
		if err != nil {
			logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
			return Result{Outcome: OutcomeFailed}, fmt.Errorf("list Terraform JSON files: %w", err)
		}
		if len(matches)+len(jsonMatches) == 0 {
			err := fmt.Errorf("terraform directory has no .tf or .tf.json files: %s", absDir)
			logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
			return Result{Outcome: OutcomeFailed}, err
		}
	}
	scanID, err := newScanID()
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
		return Result{Outcome: OutcomeFailed}, fmt.Errorf("create scan ID: %w", err)
	}

	if options.Runner == nil {
		now := time.Now().UTC()
		status := report.ScanStatusNoDrift
		outcome := OutcomeNoDrift
		if options.PlanMode == terraform.PlanModeNormal {
			status = report.ScanStatusNoChanges
			outcome = OutcomeNoChanges
		}
		logger.Info(ctx, "scan completed", "directory", absDir, "outcome", string(outcome))
		return Result{Outcome: outcome, Report: report.DriftReport{
			ScanID:          scanID,
			RootID:          rootID(absDir),
			Status:          status,
			Directory:       absDir,
			PlanMode:        string(options.PlanMode),
			ResourceChanges: []report.ResourceChange{},
			StartedAt:       now,
			CompletedAt:     now,
		}}, nil
	}

	lock := options.LockBackend
	if lock == nil {
		lock = LocalFileLockBackend{}
	}
	unlock, err := lock.Acquire(absDir)
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
		return Result{Outcome: OutcomeFailed}, err
	}
	defer unlock()

	// Re-validate after lock acquire to harden TOCTOU between initial checks and Terraform execution.
	absDir, err = ValidateDirectory(options.Directory)
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", options.Directory, "error", err)
		return Result{Outcome: OutcomeFailed}, err
	}
	if options.WorkspaceRoot != "" {
		if err := validateResolvedWorkspaceRoot(absDir, options.WorkspaceRoot); err != nil {
			logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
			return Result{Outcome: OutcomeFailed}, err
		}
	}

	scanReport, err := runTerraformScan(ctx, options.Runner, absDir, scanID, options.PlanMode, options.SkipInit)
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", absDir, "error", err)
		return Result{Outcome: OutcomeFailed, Report: scanReport}, err
	}
	if scanReport.TotalChangedResources > 0 {
		if options.PlanMode == terraform.PlanModeNormal {
			scanReport.Status = report.ScanStatusChangesDetected
			logger.Info(ctx, "scan completed", "directory", absDir, "outcome", string(OutcomeChangesDetected))
			return Result{Outcome: OutcomeChangesDetected, Report: scanReport}, nil
		}
		scanReport.Status = report.ScanStatusDriftDetected
		logger.Info(ctx, "scan completed", "directory", absDir, "outcome", string(OutcomeDriftDetected))
		return Result{Outcome: OutcomeDriftDetected, Report: scanReport}, nil
	}
	if options.PlanMode == terraform.PlanModeNormal {
		scanReport.Status = report.ScanStatusNoChanges
		logger.Info(ctx, "scan completed", "directory", absDir, "outcome", string(OutcomeNoChanges))
		return Result{Outcome: OutcomeNoChanges, Report: scanReport}, nil
	}
	scanReport.Status = report.ScanStatusNoDrift
	logger.Info(ctx, "scan completed", "directory", absDir, "outcome", string(OutcomeNoDrift))
	return Result{Outcome: OutcomeNoDrift, Report: scanReport}, nil
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

func runTerraformScan(ctx context.Context, runner terraform.Runner, directory string, scanID string, mode terraform.PlanMode, skipInit bool) (scanReport report.DriftReport, returnErr error) {
	startedAt := time.Now().UTC()
	scanReport = report.DriftReport{
		ScanID:          scanID,
		RootID:          rootID(directory),
		Status:          report.ScanStatusRunning,
		Directory:       directory,
		PlanMode:        string(mode),
		ResourceChanges: []report.ResourceChange{},
		StartedAt:       startedAt,
	}

	if !skipInit {
		logger.Info(ctx, "terraform init", "directory", directory)
		if err := runner.Init(ctx, directory); err != nil {
			failReport(&scanReport, err)
			return scanReport, fmt.Errorf("terraform init: %s", scanReport.ErrorMessage)
		}
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

	logger.Info(ctx, "terraform plan", "directory", directory, "plan_mode", string(mode))
	exitCode, err := runner.Plan(ctx, directory, planFile, mode)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, fmt.Errorf("terraform %s plan: %s", mode, scanReport.ErrorMessage)
	}
	if exitCode != 0 && exitCode != 2 {
		err := fmt.Errorf("terraform %s plan failed with exit code %d", mode, exitCode)
		failReport(&scanReport, err)
		return scanReport, err
	}

	logger.Info(ctx, "terraform show", "directory", directory)
	planJSON, err := runner.ShowJSON(ctx, directory, planFile)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, fmt.Errorf("terraform show JSON: %s", scanReport.ErrorMessage)
	}

	logger.Info(ctx, "parse plan", "directory", directory)
	resourceChanges, outputChanges, totalResources, resourcesExact, err := parser.ParsePlan(planJSON, mode)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, errors.New(scanReport.ErrorMessage)
	}

	scanReport.ResourceChanges = resourceChanges
	scanReport.OutputChanges = outputChanges
	scanReport.TotalResourcesChecked = totalResources
	scanReport.ResourcesCheckedExact = resourcesExact
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
