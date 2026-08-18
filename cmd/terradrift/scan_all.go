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

	"github.com/niravraychura/terradrift/internal/auditlog"
	"github.com/niravraychura/terradrift/internal/command"
	"github.com/niravraychura/terradrift/internal/config"
	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/spf13/cobra"
)

// reportEnrichmentOptions holds per-root report annotations applied before finalize.
type reportEnrichmentOptions struct {
	BaselineRules       []report.IgnoreRule
	IgnoreRules         []report.IgnoreRule
	ResourceOwners      map[string]string
	RemediationRunbooks map[string]string
	ApprovalFile        string
	AuditLogPath        string
	ConfigPath          string
	TerraformExec       bool
	TerraformBin        string
	PolicyCommand       string
}

type scanAllParams struct {
	Specs        []manifestRoot
	Options      scanner.Options
	Defaults     rootDefaults
	Concurrency  int
	RedactPaths  bool
	CostCommand  string
	CostArgs     []string
	AuditCommand string
	AuditArgs    []string
	Enrichment   reportEnrichmentOptions
	Delivery     deliveryOptions
}

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
	var remediationRunbooks map[string]string
	var baselineRules []report.IgnoreRule
	var ignoreRules []report.IgnoreRule
	var resourceOwners map[string]string
	var ownerWebhooks map[string]string
	var notificationThrottle bool
	var githubRepository string
	var githubPR int
	var githubIssueAfter int
	var artifactURL string
	var approvalFile string
	var auditLogPath string

	cmd := &cobra.Command{
		Use:   "scan-all",
		Short: "Scan Terraform roots from a manifest",
		Long: `Scan multiple Terraform roots from a text or JSON manifest, or by discovery.

Text manifests list one root directory per line. JSON manifests (version 1) can set
per-root profile, plan_mode, workspace, var_files, and vars. Named profiles require --config.

Delivery matches scan per root: history, dashboard HTML, slack/teams/webhook notifications,
owner webhooks, policy publish gate, cost/audit enrichment, ignore/baseline rules, owners,
runbooks, approvals, GitHub PR/issue summaries, --artifact-url, --audit-log, notification
throttle (via config), attribute-values, workspace/var-file defaults, and --failure-severity.

Prefer terradrift dashboard-index for multi-root HTML. A shared --dashboard-html path is
overwritten by the last successful root when concurrency > 1. Shared --github-pr posts one
comment per root; prefer upsert (or scan) when that is noisy.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (manifest == "") == (discover == "") {
				return fmt.Errorf("provide exactly one of --manifest or --discover")
			}
			if scanConfigPath != "" {
				cfg, err := config.Load(scanConfigPath)
				if err != nil {
					return err
				}
				if err := applyConfigOverlay(cmd, []configFlagOverlay{
					{flag: "output", assign: func() error { format = cfg.Output; return nil }},
					{flag: "timeout", assign: func() error {
						if cfg.Timeout == "" {
							return nil
						}
						parsedTimeout, err := time.ParseDuration(cfg.Timeout)
						if err != nil {
							return fmt.Errorf("parse config timeout: %w", err)
						}
						timeout = parsedTimeout
						return nil
					}},
					{flag: "redact-paths", assign: func() error { redactPaths = cfg.RedactPaths; return nil }},
					{flag: "terraform-exec", assign: func() error { terraformExec = cfg.TerraformExec; return nil }},
					{flag: "terraform-bin", assign: func() error { terraformBin = cfg.TerraformBin; return nil }},
					{flag: "plan-mode", assign: func() error { planMode = cfg.PlanMode; return nil }},
					{flag: "workspace-root", assign: func() error { workspaceRoot = cfg.WorkspaceRoot; return nil }},
					{flag: "notify", assign: func() error { notifyTarget = cfg.Notify; return nil }},
					{flag: "slack-webhook-url", assign: func() error { slackWebhookURL = cfg.SlackWebhookURL; return nil }},
					{flag: "teams-webhook-url", assign: func() error { teamsWebhookURL = cfg.TeamsWebhookURL; return nil }},
					{flag: "webhook-url", assign: func() error { webhookURL = cfg.WebhookURL; return nil }},
					{flag: "webhook-ca-cert", assign: func() error { webhookCACert = cfg.WebhookCACert; return nil }},
					{flag: "dashboard-html", assign: func() error { dashboardHTMLPath = cfg.DashboardHTML; return nil }},
					{flag: "history-dir", assign: func() error { historyDir = cfg.HistoryDir; return nil }},
					{flag: "history-retention", assign: func() error { historyRetention = cfg.HistoryRetention; return nil }},
					{flag: "history-compressed", assign: func() error { historyCompressed = cfg.HistoryCompressed; return nil }},
					{flag: "audit-log", assign: func() error { auditLogPath = cfg.AuditLog; return nil }},
					{flag: "policy-command", assign: func() error { policyCommand = cfg.PolicyCommand; return nil }},
					{flag: "policy-arg", assign: func() error { policyArgs = append([]string(nil), cfg.PolicyArgs...); return nil }},
					{flag: "cost-command", assign: func() error { costCommand = cfg.CostCommand; return nil }},
					{flag: "cost-arg", assign: func() error { costArgs = append([]string(nil), cfg.CostArgs...); return nil }},
					{assign: func() error { remediationRunbooks = cfg.RemediationRunbooks; return nil }},
					{assign: func() error { baselineRules = cfg.BaselineRules; return nil }},
					{assign: func() error { ignoreRules = cfg.IgnoreRules; return nil }},
					{assign: func() error { resourceOwners = cfg.ResourceOwners; return nil }},
					{assign: func() error { ownerWebhooks = cfg.OwnerWebhooks; return nil }},
					{assign: func() error { notificationThrottle = cfg.NotificationThrottle; return nil }},
					{flag: "github-repository", assign: func() error { githubRepository = cfg.GitHubRepository; return nil }},
					{flag: "github-pr", assign: func() error { githubPR = cfg.GitHubPR; return nil }},
					{flag: "github-issue-after", assign: func() error { githubIssueAfter = cfg.GitHubIssueAfter; return nil }},
					{flag: "artifact-url", assign: func() error { artifactURL = cfg.ArtifactURL; return nil }},
					{flag: "audit-command", assign: func() error { auditCommand = cfg.AuditCommand; return nil }},
					{flag: "audit-arg", assign: func() error { auditArgs = append([]string(nil), cfg.AuditArgs...); return nil }},
					{assign: func() error { allowedCommands = append([]string(nil), cfg.AllowedCommands...); return nil }},
					{assign: func() error { trustedCommandDirs = append([]string(nil), cfg.TrustedCommandDirs...); return nil }},
					{flag: "failure-severity", assign: func() error { failureSeverity = cfg.FailureSeverity; return nil }},
					{flag: "attribute-values", assign: func() error { attributeValues = cfg.AttributeValues; return nil }},
					{flag: "workspace", assign: func() error { terraformWorkspace = cfg.Workspace; return nil }},
					{flag: "var-file", assign: func() error { varFiles = append([]string(nil), cfg.VarFiles...); return nil }},
					{flag: "var", assign: func() error { vars = append([]string(nil), cfg.Vars...); return nil }},
				}); err != nil {
					return err
				}
			}
			if incrementalState != "" {
				normalized, err := normalizeOutputPath(incrementalState)
				if err != nil {
					return err
				}
				incrementalState = normalized
			}
			for _, path := range []*string{&dashboardHTMLPath, &historyDir, &auditLogPath} {
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
			if githubPR > 0 && githubRepository == "" {
				return fmt.Errorf("github-repository is required with github-pr")
			}
			if githubIssueAfter > 0 && (githubIssueAfter < 2 || githubRepository == "" || historyDir == "") {
				return fmt.Errorf("github-issue-after requires github-repository, history-dir, and a value of at least 2")
			}
			if strings.EqualFold(strings.TrimSpace(notifyTarget), "github") || githubPR > 0 || githubIssueAfter >= 2 {
				if strings.TrimSpace(os.Getenv("GITHUB_TOKEN")) == "" {
					return fmt.Errorf("GITHUB_TOKEN is required when GitHub notification delivery is configured")
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
			sideEffectMu := &sync.Mutex{}
			delivery := deliveryOptions{
				AttributeValues:      attributeValues,
				ArtifactURL:          artifactURL,
				HistoryDir:           historyDir,
				HistoryRetention:     historyRetention,
				HistoryCompressed:    historyCompressed,
				DashboardHTMLPath:    dashboardHTMLPath,
				PolicyCommand:        policyCommand,
				PolicyArgs:           policyArgs,
				NotifyTarget:         notifyTarget,
				SlackWebhookURL:      slackWebhookURL,
				TeamsWebhookURL:      teamsWebhookURL,
				WebhookURL:           webhookURL,
				WebhookCACert:        webhookCACert,
				GitHubRepository:     githubRepository,
				GitHubPR:             githubPR,
				GitHubIssueAfter:     githubIssueAfter,
				OwnerWebhooks:        ownerWebhooks,
				NotificationThrottle: notificationThrottle,
				historyMu:            sideEffectMu,
			}
			defaults := rootDefaults{
				PlanMode:  planMode,
				Workspace: terraformWorkspace,
				VarFiles:  append([]string(nil), varFiles...),
				Vars:      append([]string(nil), vars...),
				Config:    scanConfigPath,
			}
			aggregate := scanAll(cmd.Context(), scanAllParams{
				Specs:        roots,
				Options:      options,
				Defaults:     defaults,
				Concurrency:  concurrency,
				RedactPaths:  redactPaths,
				CostCommand:  costCommand,
				CostArgs:     costArgs,
				AuditCommand: auditCommand,
				AuditArgs:    auditArgs,
				Enrichment: reportEnrichmentOptions{
					BaselineRules:       baselineRules,
					IgnoreRules:         ignoreRules,
					ResourceOwners:      resourceOwners,
					RemediationRunbooks: remediationRunbooks,
					ApprovalFile:        approvalFile,
					AuditLogPath:        auditLogPath,
					ConfigPath:          scanConfigPath,
					TerraformExec:       terraformExec,
					TerraformBin:        terraformBin,
					PolicyCommand:       policyCommand,
				},
				Delivery: delivery,
			})
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
	cmd.Flags().StringVar(&githubRepository, "github-repository", "", "GitHub repository for pull request summary (owner/repo)")
	cmd.Flags().IntVar(&githubPR, "github-pr", 0, "GitHub pull request number for scan summary (one comment per root)")
	cmd.Flags().IntVar(&githubIssueAfter, "github-issue-after", 0, "create a GitHub issue after this many consecutive matching drift scans per root")
	cmd.Flags().StringVar(&artifactURL, "artifact-url", "", "presigned HTTPS URL to upload each root JSON report")
	cmd.Flags().StringVar(&approvalFile, "approval-file", "", "review-only approval artifact to attach to each root report")
	cmd.Flags().StringVar(&auditLogPath, "audit-log", "", "append secret-safe JSON audit events per root to this path")
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
	cmd.Flags().StringVar(&scanConfigPath, "config", "", "config file for delivery defaults and per-root profile names")
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

func scanAll(ctx context.Context, params scanAllParams) multiScanReport {
	specs := params.Specs
	roots := make([]multiScanRoot, len(specs))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(params.Concurrency, len(specs)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				spec := specs[index]
				root := multiScanRoot{Directory: spec.Directory}
				rootOptions, resolveErr := resolveRootOptions(spec, params.Defaults, params.Options)
				if resolveErr != nil {
					if params.RedactPaths {
						root.Directory = "[REDACTED]"
						root.Error = "scan failed"
					} else {
						root.Error = resolveErr.Error()
					}
					appendScanAllAudit(params, root, spec.Profile, resolveErr)
					roots[index] = root
					continue
				}
				result, err := scanner.Scan(ctx, rootOptions)
				if err != nil {
					if params.RedactPaths {
						root.Directory = "[REDACTED]"
						root.Error = "scan failed"
					} else {
						root.Error = err.Error()
					}
					appendScanAllAudit(params, root, spec.Profile, err)
				} else {
					scanReport := result.Report
					processErr := enrichAndFinalizeRoot(ctx, &scanReport, params)
					if processErr != nil {
						root.Error = processErr.Error()
						appendScanAllAudit(params, multiScanRoot{Directory: root.Directory, Report: scanReport, Error: processErr.Error()}, spec.Profile, processErr)
					} else {
						if params.RedactPaths {
							scanReport.Directory = "[REDACTED]"
						}
						root.Report = scanReport
						appendScanAllAudit(params, root, spec.Profile, nil)
					}
					if params.RedactPaths {
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

func enrichAndFinalizeRoot(ctx context.Context, scanReport *report.DriftReport, params scanAllParams) error {
	rules := append(append([]report.IgnoreRule(nil), params.Enrichment.BaselineRules...), params.Enrichment.IgnoreRules...)
	if err := report.ApplyIgnoreRules(scanReport, rules); err != nil {
		return err
	}
	report.ApplyOwners(scanReport, params.Enrichment.ResourceOwners)
	enriched, err := enrichReport(ctx, *scanReport, params.CostCommand, params.CostArgs, params.AuditCommand, params.AuditArgs)
	if err != nil {
		return err
	}
	*scanReport = enriched
	if err := report.ApplyRunbooks(scanReport, params.Enrichment.RemediationRunbooks); err != nil {
		return err
	}
	if params.Enrichment.ApprovalFile != "" {
		data, err := ioutil.ReadLimitedFile(params.Enrichment.ApprovalFile, int64(maxApprovalBytes))
		if err != nil {
			return fmt.Errorf("read approval %s: %w", params.Enrichment.ApprovalFile, err)
		}
		var approval report.Approval
		if err := json.Unmarshal(data, &approval); err != nil {
			return fmt.Errorf("parse approval %s: %w", params.Enrichment.ApprovalFile, err)
		}
		if err := report.VerifyApproval(*scanReport, approval); err != nil {
			return err
		}
		scanReport.Approval = &approval
	}
	return finalizeRootScan(ctx, *scanReport, params.Delivery)
}

func appendScanAllAudit(params scanAllParams, root multiScanRoot, profile string, runErr error) {
	if params.Enrichment.AuditLogPath == "" {
		return
	}
	event := auditlog.Event{
		Event:            "scan_completed",
		ScanID:           root.Report.ScanID,
		Status:           string(root.Report.Status),
		PlanMode:         root.Report.PlanMode,
		Workspace:        filepath.Base(root.Directory),
		Config:           filepath.Base(params.Enrichment.ConfigPath),
		Profile:          profile,
		TerraformVersion: root.Report.TerraformVersion,
		Commands:         auditCommandNames(params.Enrichment.TerraformExec, params.Enrichment.TerraformBin, params.CostCommand, params.Enrichment.PolicyCommand, params.AuditCommand),
	}
	if runErr != nil || root.Error != "" {
		event.Event = "scan_failed"
		if runErr != nil {
			event.Error = runErr.Error()
		} else {
			event.Error = root.Error
		}
	}
	_ = withHistoryLock(params.Delivery.historyMu, func() error {
		return auditlog.Append(params.Enrichment.AuditLogPath, event)
	})
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
