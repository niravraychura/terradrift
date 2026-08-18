package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/niravraychura/terradrift/internal/command"
	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/spf13/cobra"
)

func newScanAllCommand(stdout io.Writer) *cobra.Command {
	var manifest string
	var discover string
	var includes []string
	var excludes []string
	var format string
	var timeout time.Duration
	var concurrency int
	var terraformExec bool
	var terraformBin string
	var workspaceRoot string
	var redactPaths bool
	var incrementalState string
	var planMode string
	var lockBackendName string
	var skipTerraformInit bool
	var historyDir string
	var historyRetention int
	var historyCompressed bool
	var dashboardHTMLPath string
	var notifyTarget string
	var slackWebhookURL string
	var teamsWebhookURL string
	var webhookURL string
	var webhookCACert string
	var policyCommand string
	var policyArgs []string
	var costCommand string
	var costArgs []string
	var auditCommand string
	var auditArgs []string
	var allowedCommands []string
	var trustedCommandDirs []string
	var attributeValues bool
	var terraformWorkspace string
	var varFiles []string
	var vars []string
	var scanConfigPath string
	var failureSeverity string

	cmd := &cobra.Command{
		Use:   "scan-all",
		Short: "Scan Terraform roots from a manifest",
		Long: `Scan multiple Terraform roots from a text or JSON manifest, or by discovery.

Text manifests list one root directory per line. JSON manifests (version 1) can set
per-root profile, plan_mode, workspace, var_files, and vars. Named profiles require --config.

Delivery subset (production-usable): history, dashboard HTML, notifications (slack/teams/webhook),
policy publish gate, cost/audit enrichment, attribute-values, workspace/var-file defaults,
and --failure-severity for drift exit gating.

Not yet supported on scan-all (use terradrift scan per root): baselines/owners/runbooks,
GitHub PR/issue summaries, --artifact-url, --audit-log, notification throttle, approvals.

Prefer terradrift dashboard-index for multi-root HTML. A shared --dashboard-html path is
overwritten by the last successful root when concurrency > 1.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (manifest == "") == (discover == "") {
				return fmt.Errorf("provide exactly one of --manifest or --discover")
			}
			if incrementalState != "" {
				normalized, err := normalizeOutputPath(incrementalState)
				if err != nil {
					return err
				}
				incrementalState = normalized
			}
			for _, path := range []*string{&dashboardHTMLPath, &historyDir} {
				if *path == "" {
					continue
				}
				normalized, err := normalizeOutputPath(*path)
				if err != nil {
					return err
				}
				*path = normalized
			}
			var roots []manifestRoot
			var err error
			if manifest != "" {
				roots, err = loadScanManifest(manifest)
			} else {
				directories, discoverErr := discoverTerraformRoots(discover, includes, excludes)
				if discoverErr != nil {
					return discoverErr
				}
				roots = make([]manifestRoot, 0, len(directories))
				for _, directory := range directories {
					roots = append(roots, manifestRoot{Directory: directory})
				}
			}
			if err != nil {
				return err
			}
			if incrementalState != "" {
				directories, err := incrementalRoots(incrementalState, directoriesFromRoots(roots))
				if err != nil {
					return err
				}
				roots = filterRoots(roots, directories)
			}
			parsedFormat, err := parseOutputFormat(format)
			if err != nil {
				return err
			}
			if parsedFormat != outputFormatTable && parsedFormat != outputFormatJSON {
				return fmt.Errorf("scan-all supports table and json output")
			}
			if concurrency <= 0 {
				return fmt.Errorf("concurrency must be greater than zero")
			}
			for _, external := range []string{costCommand, policyCommand, auditCommand} {
				if external != "" {
					if err := command.Validate(external, allowedCommands, trustedCommandDirs); err != nil {
						return err
					}
				}
			}

			mode, err := terraform.ParsePlanMode(planMode)
			if err != nil {
				return err
			}
			lockBackend, err := scanner.ParseLockBackend(lockBackendName)
			if err != nil {
				return err
			}
			options := scanner.Options{
				Timeout:       timeout,
				WorkspaceRoot: workspaceRoot,
				PlanMode:      mode,
				LockBackend:   lockBackend,
				SkipInit:      skipTerraformInit,
			}
			options, err = scanner.PrepareOptions(options)
			if err != nil {
				return err
			}
			if terraformExec {
				options.RequireTerraformFiles = true
				runner := terraform.NewCLIRunner(terraformBin)
				runner.Workspace = terraformWorkspace
				runner.VarFiles = append([]string(nil), varFiles...)
				runner.Vars = append([]string(nil), vars...)
				options.Runner = runner
			} else {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: bootstrap report only; pass --terraform-exec for a real drift scan")
			}
			if dashboardHTMLPath != "" && concurrency > 1 {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: shared --dashboard-html is overwritten by the last successful root; prefer terradrift dashboard-index for multi-root views")
			}
			delivery := deliveryOptions{
				AttributeValues:   attributeValues,
				HistoryDir:        historyDir,
				HistoryRetention:  historyRetention,
				HistoryCompressed: historyCompressed,
				DashboardHTMLPath: dashboardHTMLPath,
				PolicyCommand:     policyCommand,
				PolicyArgs:        policyArgs,
				NotifyTarget:      notifyTarget,
				SlackWebhookURL:   slackWebhookURL,
				TeamsWebhookURL:   teamsWebhookURL,
				WebhookURL:        webhookURL,
				WebhookCACert:     webhookCACert,
				historyMu:         &sync.Mutex{},
			}
			defaults := rootDefaults{
				PlanMode:  planMode,
				Workspace: terraformWorkspace,
				VarFiles:  append([]string(nil), varFiles...),
				Vars:      append([]string(nil), vars...),
				Config:    scanConfigPath,
			}
			aggregate := scanAll(cmd.Context(), roots, options, defaults, concurrency, redactPaths, costCommand, costArgs, auditCommand, auditArgs, delivery)
			if err := writeMultiScanReport(stdout, aggregate, parsedFormat); err != nil {
				return err
			}
			if incrementalState != "" {
				if err := writeIncrementalState(incrementalState, aggregate); err != nil {
					return err
				}
			}
			if aggregate.FailedRoots > 0 {
				return fmt.Errorf("%w: %d roots", errMultiScanFailed, aggregate.FailedRoots)
			}
			if aggregate.DriftedRoots > 0 {
				if failureSeverity == "" {
					return errDriftDetected
				}
				meets, err := multiScanMeetsSeverity(aggregate, failureSeverity)
				if err != nil {
					return err
				}
				if meets {
					return errDriftDetected
				}
			}
			if aggregate.ChangedRoots > 0 {
				return errChangesDetected
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&manifest, "manifest", "", "text or JSON Terraform root manifest")
	cmd.Flags().StringVar(&discover, "discover", "", "workspace root to discover Terraform roots")
	cmd.Flags().StringArrayVar(&includes, "include", nil, "root-relative include pattern; repeatable")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "root-relative exclude pattern; repeatable")
	cmd.Flags().StringVarP(&format, "output", "o", string(outputFormatTable), "output format: table, json")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "maximum scan duration per root")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "maximum concurrent scans")
	cmd.Flags().BoolVar(&terraformExec, "terraform-exec", false, "run Terraform-compatible scans")
	cmd.Flags().StringVar(&planMode, "plan-mode", string(terraform.PlanModeRefreshOnly), "default plan mode: refresh-only or normal (overridable per root)")
	cmd.Flags().StringVar(&terraformBin, "terraform-bin", "", "Terraform-compatible executable to run (default: terraform)")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "require roots to resolve inside this workspace root")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact local filesystem paths from scan output")
	cmd.Flags().StringVar(&incrementalState, "incremental-state", "", "JSON state file; retry only roots previously drifted or failed")
	cmd.Flags().StringVar(&lockBackendName, "lock-backend", "local", "scan lock backend: local (single-host file lock)")
	cmd.Flags().BoolVar(&skipTerraformInit, "skip-terraform-init", false, "skip terraform init when .terraform is already valid")
	cmd.Flags().StringVar(&historyDir, "history-dir", "", "write per-root JSON scan history to this directory")
	cmd.Flags().IntVar(&historyRetention, "history-retention", 0, "maximum history reports to retain (0 keeps all)")
	cmd.Flags().BoolVar(&historyCompressed, "history-compressed", false, "store history reports as gzip-compressed JSON")
	cmd.Flags().StringVar(&dashboardHTMLPath, "dashboard-html", "", "write last successful root dashboard HTML (prefer dashboard-index for multi-root)")
	cmd.Flags().StringVar(&notifyTarget, "notify", "", "notification target: slack, teams, webhook")
	cmd.Flags().StringVar(&slackWebhookURL, "slack-webhook-url", "", "Slack incoming webhook URL")
	cmd.Flags().StringVar(&teamsWebhookURL, "teams-webhook-url", "", "Microsoft Teams incoming webhook URL")
	cmd.Flags().StringVar(&webhookURL, "webhook-url", "", "generic HTTPS webhook URL")
	cmd.Flags().StringVar(&webhookCACert, "webhook-ca-cert", "", "PEM CA certificate file for webhook TLS verification")
	cmd.Flags().StringVar(&policyCommand, "policy-command", "", "policy command run per root before history/notify")
	cmd.Flags().StringArrayVar(&policyArgs, "policy-arg", nil, "policy command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&costCommand, "cost-command", "", "cost command to enrich each root report")
	cmd.Flags().StringArrayVar(&costArgs, "cost-arg", nil, "cost command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&auditCommand, "audit-command", "", "audit correlation command to enrich each root report")
	cmd.Flags().StringArrayVar(&auditArgs, "audit-arg", nil, "audit command argument; repeat for multiple arguments")
	cmd.Flags().BoolVar(&attributeValues, "attribute-values", false, "include safe attribute values in persisted/automation output")
	cmd.Flags().StringVar(&terraformWorkspace, "workspace", "", "default Terraform workspace (overridable per root)")
	cmd.Flags().StringArrayVar(&varFiles, "var-file", nil, "default Terraform -var-file path; repeatable (overridable per root)")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "default Terraform -var assignment; repeatable (overridable per root)")
	cmd.Flags().StringVar(&scanConfigPath, "config", "", "config file used to resolve per-root profile names")
	cmd.Flags().StringVar(&failureSeverity, "failure-severity", "", "minimum drift severity that fails the multi-root scan: low, medium, high, critical")
	return cmd
}

func multiScanMeetsSeverity(aggregate multiScanReport, threshold string) (bool, error) {
	for _, root := range aggregate.Roots {
		if root.Error != "" || root.Report.Status != report.ScanStatusDriftDetected {
			continue
		}
		meets, err := report.MeetsSeverity(root.Report, threshold)
		if err != nil {
			return false, err
		}
		if meets {
			return true, nil
		}
	}
	return false, nil
}

type multiScanReport struct {
	Status                multiScanStatus `json:"status"`
	Roots                 []multiScanRoot `json:"roots"`
	TotalRoots            int             `json:"total_roots"`
	DriftedRoots          int             `json:"drifted_roots"`
	ChangedRoots          int             `json:"changed_roots"`
	FailedRoots           int             `json:"failed_roots"`
	TotalResourcesChecked int             `json:"total_resources_checked"`
	TotalChangedResources int             `json:"total_changed_resources"`
}

type multiScanStatus string

const (
	multiScanStatusComplete        multiScanStatus = "complete"
	multiScanStatusDriftDetected   multiScanStatus = "drift_detected"
	multiScanStatusChangesDetected multiScanStatus = "changes_detected"
	multiScanStatusPartial         multiScanStatus = "partial"
	multiScanStatusFailed          multiScanStatus = "failed"
)

type multiScanRoot struct {
	Directory string             `json:"directory"`
	Report    report.DriftReport `json:"report,omitempty"`
	Error     string             `json:"error,omitempty"`
}

type incrementalState struct {
	Version int                         `json:"version"`
	Roots   map[string]incrementalEntry `json:"roots"`
}

type incrementalEntry struct {
	Status    multiScanStatus `json:"status"`
	ScanID    string          `json:"scan_id,omitempty"`
	Completed time.Time       `json:"completed_at,omitempty"`
}

func incrementalRoots(path string, directories []string) ([]string, error) {
	data, err := ioutil.ReadLimitedFile(path, int64(maxManifestBytes))
	if os.IsNotExist(err) {
		return directories, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read incremental state %s: %w", path, err)
	}
	var state incrementalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse incremental state %s: %w", path, err)
	}
	if state.Version != 1 {
		return nil, fmt.Errorf("unsupported incremental state version %d", state.Version)
	}
	roots := make([]string, 0, len(directories))
	for _, directory := range directories {
		entry, found := state.Roots[directory]
		if !found || entry.Status != multiScanStatusComplete {
			roots = append(roots, directory)
		}
	}
	return roots, nil
}

func writeIncrementalState(path string, aggregate multiScanReport) error {
	if err := rejectSymlink(path); err != nil {
		return err
	}
	state := incrementalState{Version: 1, Roots: make(map[string]incrementalEntry, len(aggregate.Roots))}
	if data, err := ioutil.ReadLimitedFile(path, int64(maxManifestBytes)); err == nil {
		if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 {
			return fmt.Errorf("parse incremental state %s", path)
		}
		if state.Roots == nil {
			state.Roots = make(map[string]incrementalEntry)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read incremental state %s: %w", path, err)
	}
	for _, root := range aggregate.Roots {
		entry := incrementalEntry{Status: multiScanStatusComplete}
		if root.Error != "" {
			entry.Status = multiScanStatusFailed
		} else if root.Report.Status == report.ScanStatusDriftDetected {
			entry.Status = multiScanStatusDriftDetected
		}
		entry.ScanID = root.Report.ScanID
		entry.Completed = root.Report.CompletedAt
		state.Roots[root.Directory] = entry
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode incremental state: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create incremental state directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".terradrift-state-*")
	if err != nil {
		return fmt.Errorf("create incremental state: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure incremental state: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write incremental state: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close incremental state: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace incremental state: %w", err)
	}
	return nil
}

func discoverTerraformRoots(root string, includes []string, excludes []string) ([]string, error) {
	for _, pattern := range append(append([]string{}, includes...), excludes...) {
		if _, err := filepath.Match(filepath.Clean(pattern), ""); err != nil {
			return nil, fmt.Errorf("invalid discovery pattern %q: %w", pattern, err)
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve discovery root: %w", err)
	}
	roots := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".terraform" || (relative != "." && matchesPath(relative, excludes)) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".tf" {
			return nil
		}
		directory := filepath.Dir(path)
		relative, err = filepath.Rel(root, directory)
		if err != nil {
			return err
		}
		if matchesPath(relative, excludes) || (len(includes) > 0 && !matchesPath(relative, includes)) {
			return nil
		}
		roots[directory] = true
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Terraform roots: %w", err)
	}
	directories := make([]string, 0, len(roots))
	for directory := range roots {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	if len(directories) == 0 {
		return nil, fmt.Errorf("no Terraform roots found under %s", root)
	}
	return directories, nil
}

func matchesPath(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = filepath.Clean(pattern)
		if path == pattern || strings.HasPrefix(path, pattern+string(filepath.Separator)) {
			return true
		}
		if matched, err := filepath.Match(pattern, path); err == nil && matched {
			return true
		}
	}
	return false
}

func scanAll(ctx context.Context, specs []manifestRoot, options scanner.Options, defaults rootDefaults, concurrency int, redactPaths bool, costCommand string, costArgs []string, auditCommand string, auditArgs []string, delivery deliveryOptions) multiScanReport {
	roots := make([]multiScanRoot, len(specs))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(concurrency, len(specs)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				spec := specs[index]
				root := multiScanRoot{Directory: spec.Directory}
				rootOptions, resolveErr := resolveRootOptions(spec, defaults, options)
				if resolveErr != nil {
					if redactPaths {
						root.Directory = "[REDACTED]"
						root.Error = "scan failed"
					} else {
						root.Error = resolveErr.Error()
					}
					roots[index] = root
					continue
				}
				result, err := scanner.Scan(ctx, rootOptions)
				if err != nil {
					if redactPaths {
						root.Directory = "[REDACTED]"
						root.Error = "scan failed"
					} else {
						root.Error = err.Error()
					}
				} else {
					scanReport := result.Report
					enriched, enrichErr := enrichReport(ctx, scanReport, costCommand, costArgs, auditCommand, auditArgs)
					if enrichErr != nil {
						root.Error = enrichErr.Error()
					} else {
						scanReport = enriched
						if redactPaths {
							scanReport.Directory = "[REDACTED]"
						}
						if finalizeErr := finalizeRootScan(ctx, scanReport, delivery); finalizeErr != nil {
							root.Error = finalizeErr.Error()
						} else {
							root.Report = scanReport
						}
					}
					if redactPaths {
						root.Directory = "[REDACTED]"
					}
				}
				roots[index] = root
			}
		}()
	}
	for index := range specs {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	aggregate := multiScanReport{Roots: roots, TotalRoots: len(roots)}
	for _, root := range roots {
		if root.Error != "" {
			aggregate.FailedRoots++
			continue
		}
		aggregate.TotalResourcesChecked += root.Report.TotalResourcesChecked
		aggregate.TotalChangedResources += root.Report.TotalChangedResources
		switch root.Report.Status {
		case report.ScanStatusDriftDetected:
			aggregate.DriftedRoots++
		case report.ScanStatusChangesDetected:
			aggregate.ChangedRoots++
		}
	}
	aggregate.Status = multiScanStatusFor(aggregate.TotalRoots, aggregate.DriftedRoots, aggregate.ChangedRoots, aggregate.FailedRoots)
	return aggregate
}

func multiScanStatusFor(totalRoots, driftedRoots, changedRoots, failedRoots int) multiScanStatus {
	if failedRoots == totalRoots {
		return multiScanStatusFailed
	}
	if failedRoots > 0 {
		return multiScanStatusPartial
	}
	if driftedRoots > 0 {
		return multiScanStatusDriftDetected
	}
	if changedRoots > 0 {
		return multiScanStatusChangesDetected
	}
	return multiScanStatusComplete
}
