// Package scanner orchestrates TerraDrift scan workflows.
package scanner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
const maxPlanFileBytes int64 = 32 << 20

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
	RedactPaths           bool
	PlanFile              string
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

func logDirectory(redact bool, directory string) string {
	if redact {
		return "[REDACTED]"
	}
	return directory
}

func logError(redactPaths bool, err error, paths ...string) error {
	if !redactPaths || err == nil {
		return err
	}
	msg := err.Error()
	for _, path := range paths {
		if path != "" {
			msg = strings.ReplaceAll(msg, path, "[REDACTED]")
			if absPath, err := filepath.Abs(path); err == nil && absPath != path {
				msg = strings.ReplaceAll(msg, absPath, "[REDACTED]")
			}
		}
	}
	return errors.New(msg)
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
		logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, options.Directory), "error", logError(options.RedactPaths, err, options.Directory))
		return Result{Outcome: OutcomeFailed}, err
	}
	logger.Info(ctx, "scan started", "directory", logDirectory(options.RedactPaths, absDir))
	if options.WorkspaceRoot != "" {
		if err := validateResolvedWorkspaceRoot(absDir, options.WorkspaceRoot); err != nil {
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir, options.WorkspaceRoot))
			return Result{Outcome: OutcomeFailed}, err
		}
	}
	if options.RequireTerraformFiles {
		ok, err := hasPlannerFiles(absDir)
		if err != nil {
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir))
			return Result{Outcome: OutcomeFailed}, err
		}
		if !ok {
			err := fmt.Errorf("terraform directory has no .tf, .tf.json, or terragrunt.hcl files: %s", absDir)
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir))
			return Result{Outcome: OutcomeFailed}, err
		}
	}
	scanID, err := newScanID()
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir))
		return Result{Outcome: OutcomeFailed}, fmt.Errorf("create scan ID: %w", err)
	}

	if strings.TrimSpace(options.PlanFile) != "" {
		if options.Runner == nil {
			err := fmt.Errorf("--plan-file requires --terraform-exec")
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir, options.PlanFile))
			return Result{Outcome: OutcomeFailed}, err
		}
		planFile, err := validatePlanFile(options.PlanFile, options.WorkspaceRoot)
		if err != nil {
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir, options.PlanFile, options.WorkspaceRoot))
			return Result{Outcome: OutcomeFailed}, err
		}
		options.PlanFile = planFile
	}

	if options.Runner == nil {
		now := time.Now().UTC()
		status := report.ScanStatusNoDrift
		outcome := OutcomeNoDrift
		if options.PlanMode == terraform.PlanModeNormal {
			status = report.ScanStatusNoChanges
			outcome = OutcomeNoChanges
		}
		logger.Info(ctx, "scan completed", "directory", logDirectory(options.RedactPaths, absDir), "outcome", string(outcome))
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
		logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir))
		return Result{Outcome: OutcomeFailed}, err
	}
	defer unlock()

	// Re-validate after lock acquire to harden TOCTOU between initial checks and Terraform execution.
	absDir, err = ValidateDirectory(options.Directory)
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, options.Directory), "error", logError(options.RedactPaths, err, options.Directory))
		return Result{Outcome: OutcomeFailed}, err
	}
	if options.WorkspaceRoot != "" {
		if err := validateResolvedWorkspaceRoot(absDir, options.WorkspaceRoot); err != nil {
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir, options.WorkspaceRoot))
			return Result{Outcome: OutcomeFailed}, err
		}
	}
	if options.PlanFile != "" {
		planFile, err := validatePlanFile(options.PlanFile, options.WorkspaceRoot)
		if err != nil {
			logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir, options.PlanFile, options.WorkspaceRoot))
			return Result{Outcome: OutcomeFailed}, err
		}
		options.PlanFile = planFile
	}

	scanReport, err := runTerraformScan(ctx, options.Runner, absDir, scanID, options.PlanMode, options.SkipInit, options.RedactPaths, options.PlanFile)
	if err != nil {
		logger.Error(ctx, "scan failed", "directory", logDirectory(options.RedactPaths, absDir), "error", logError(options.RedactPaths, err, absDir))
		return Result{Outcome: OutcomeFailed, Report: scanReport}, err
	}
	if scanReport.TotalChangedResources > 0 {
		if options.PlanMode == terraform.PlanModeNormal {
			scanReport.Status = report.ScanStatusChangesDetected
			logger.Info(ctx, "scan completed", "directory", logDirectory(options.RedactPaths, absDir), "outcome", string(OutcomeChangesDetected))
			return Result{Outcome: OutcomeChangesDetected, Report: scanReport}, nil
		}
		scanReport.Status = report.ScanStatusDriftDetected
		logger.Info(ctx, "scan completed", "directory", logDirectory(options.RedactPaths, absDir), "outcome", string(OutcomeDriftDetected))
		return Result{Outcome: OutcomeDriftDetected, Report: scanReport}, nil
	}
	if options.PlanMode == terraform.PlanModeNormal {
		scanReport.Status = report.ScanStatusNoChanges
		logger.Info(ctx, "scan completed", "directory", logDirectory(options.RedactPaths, absDir), "outcome", string(OutcomeNoChanges))
		return Result{Outcome: OutcomeNoChanges, Report: scanReport}, nil
	}
	scanReport.Status = report.ScanStatusNoDrift
	logger.Info(ctx, "scan completed", "directory", logDirectory(options.RedactPaths, absDir), "outcome", string(OutcomeNoDrift))
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

func hasPlannerFiles(directory string) (bool, error) {
	matches, err := filepath.Glob(filepath.Join(directory, "*.tf"))
	if err != nil {
		return false, fmt.Errorf("list Terraform files: %w", err)
	}
	jsonMatches, err := filepath.Glob(filepath.Join(directory, "*.tf.json"))
	if err != nil {
		return false, fmt.Errorf("list Terraform JSON files: %w", err)
	}
	if len(matches)+len(jsonMatches) > 0 {
		return true, nil
	}
	return terraform.IsTerragruntRoot(directory), nil
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

func validatePlanFile(path string, workspaceRoot string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve plan file: %w", err)
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("plan file does not exist: %s", absPath)
		}
		return "", fmt.Errorf("inspect plan file %s: %w", absPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("plan file must not be a symlink: %s", absPath)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("plan file is not a regular file: %s", absPath)
	}
	if info.Size() > maxPlanFileBytes {
		return "", fmt.Errorf("plan file exceeds %d bytes", maxPlanFileBytes)
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve plan file path: %w", err)
	}
	if workspaceRoot != "" {
		rel, err := filepath.Rel(workspaceRoot, resolved)
		if err != nil {
			return "", fmt.Errorf("compare plan file to workspace root: %w", err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", fmt.Errorf("plan file %s is outside workspace root %s", resolved, workspaceRoot)
		}
	}
	return resolved, nil
}

func runTerraformScan(ctx context.Context, runner terraform.Runner, directory string, scanID string, mode terraform.PlanMode, skipInit bool, redactPaths bool, existingPlanFile string) (scanReport report.DriftReport, returnErr error) {
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

	if existingPlanFile == "" {
		if !skipInit {
			logger.Info(ctx, "terraform init", "directory", logDirectory(redactPaths, directory))
			if err := runner.Init(ctx, directory); err != nil {
				failReport(&scanReport, err)
				return scanReport, fmt.Errorf("terraform init: %s", scanReport.ErrorMessage)
			}
		} else if err := requireInitializedTerraform(directory); err != nil {
			failReport(&scanReport, err)
			return scanReport, fmt.Errorf("skip terraform init: %s", scanReport.ErrorMessage)
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

	planFile := existingPlanFile
	if planFile == "" {
		created, cleanup, err := securePlanFile(directory)
		if err != nil {
			failReport(&scanReport, err)
			return scanReport, errors.New(scanReport.ErrorMessage)
		}
		planFile = created
		defer func() {
			if err := cleanup(); err != nil && returnErr == nil {
				failReport(&scanReport, err)
				returnErr = fmt.Errorf("remove secure terraform plan file: %s", scanReport.ErrorMessage)
			}
		}()

		logger.Info(ctx, "terraform plan", "directory", logDirectory(redactPaths, directory), "plan_mode", string(mode))
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
	}

	logger.Info(ctx, "terraform show", "directory", logDirectory(redactPaths, directory))
	planReader, err := runner.ShowJSON(ctx, directory, planFile)
	if err != nil {
		failReport(&scanReport, err)
		return scanReport, fmt.Errorf("terraform show JSON: %s", scanReport.ErrorMessage)
	}
	defer func() {
		if err := planReader.Close(); err != nil && returnErr == nil {
			failReport(&scanReport, err)
			returnErr = fmt.Errorf("terraform show JSON: %s", scanReport.ErrorMessage)
		}
	}()

	logger.Info(ctx, "parse plan", "directory", logDirectory(redactPaths, directory))
	limited := &io.LimitedReader{R: planReader, N: maxPlanFileBytes + 1}
	resourceChanges, outputChanges, totalResources, resourcesExact, err := parser.ParsePlanReader(limited, mode)
	if limited.N == 0 {
		err = fmt.Errorf("terraform show JSON exceeded %d bytes", maxPlanFileBytes)
	}
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

func requireInitializedTerraform(directory string) error {
	terraformDir := filepath.Join(directory, ".terraform")
	info, err := os.Stat(terraformDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("--skip-terraform-init requires an initialized .terraform directory")
	}
	providers := filepath.Join(terraformDir, "providers")
	if st, err := os.Stat(providers); err == nil && st.IsDir() {
		return nil
	}
	if _, err := os.Stat(filepath.Join(terraformDir, "modules", "modules.json")); err == nil {
		return nil
	}
	return fmt.Errorf("--skip-terraform-init requires .terraform/providers or .terraform/modules/modules.json")
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
