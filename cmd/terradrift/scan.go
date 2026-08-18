package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/auditlog"
	"github.com/niravraychura/terradrift/internal/command"
	"github.com/niravraychura/terradrift/internal/config"
	"github.com/niravraychura/terradrift/internal/dashboard"
	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/ioutil"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/spf13/cobra"
)

type configFlagOverlay struct {
	flag   string
	assign func() error
}

func applyConfigOverlay(cmd *cobra.Command, overlays []configFlagOverlay) error {
	for _, overlay := range overlays {
		if overlay.flag != "" && cmd.Flags().Changed(overlay.flag) {
			continue
		}
		if err := overlay.assign(); err != nil {
			return err
		}
	}
	return nil
}

func newScanCommand(stdout io.Writer) *cobra.Command {
	var directory string
	var format string
	var timeout time.Duration
	var redactPaths bool
	var terraformExec bool
	var terraformBin string
	var scanConfigPath string
	var configProfile string
	var workspaceRoot string
	var notifyTarget string
	var slackWebhookURL string
	var teamsWebhookURL string
	var webhookURL string
	var webhookCACert string
	var dashboardHTMLPath string
	var historyDir string
	var historyRetention int
	var policyCommand string
	var policyArgs []string
	var costCommand string
	var costArgs []string
	var remediationRunbooks map[string]string
	var baselineRules []report.IgnoreRule
	var ignoreRules []report.IgnoreRule
	var failureSeverity string
	var resourceOwners map[string]string
	var ownerWebhooks map[string]string
	var notificationThrottle bool
	var githubRepository string
	var githubPR int
	var githubIssueAfter int
	var artifactURL string
	var approvalFile string
	var auditCommand string
	var auditArgs []string
	var allowedCommands []string
	var trustedCommandDirs []string
	var auditLogPath string
	var historyCompressed bool
	var planMode string
	var lockBackendName string
	var skipTerraformInit bool
	var attributeValues bool
	var terraformWorkspace string
	var varFiles []string
	var vars []string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Validate a Terraform directory for drift scanning",
		Example: `  terradrift scan
  terradrift scan --directory ./terraform/prod
  terradrift scan -d ./terraform/prod --output json`,
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			var auditReport report.DriftReport
			defer func() {
				if auditLogPath == "" {
					return
				}
				event := auditlog.Event{Event: "scan_completed", ScanID: auditReport.ScanID, Status: string(auditReport.Status), PlanMode: auditReport.PlanMode, Workspace: filepath.Base(auditReport.Directory), Config: filepath.Base(scanConfigPath), Profile: configProfile, TerraformVersion: auditReport.TerraformVersion, Commands: auditCommandNames(terraformExec, terraformBin, costCommand, policyCommand, auditCommand)}
				if runErr != nil {
					event.Event = "scan_failed"
					event.Error = runErr.Error()
				}
				if err := auditlog.Append(auditLogPath, event); err != nil && runErr == nil {
					runErr = err
				}
			}()
			if scanConfigPath != "" || configProfile != "" {
				cfg, err := config.LoadProfile(scanConfigPath, configProfile)
				if err != nil {
					return err
				}
				if err := applyConfigOverlay(cmd, []configFlagOverlay{
					{flag: "directory", assign: func() error { directory = cfg.Directory; return nil }},
					{flag: "output", assign: func() error { format = cfg.Output; return nil }},
					{flag: "timeout", assign: func() error {
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

			parsedFormat, err := parseOutputFormat(format)
			if err != nil {
				return err
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
			pipelineTimeout := timeout
			if pipelineTimeout <= 0 {
				pipelineTimeout = scanner.DefaultTimeout
			}
			scanContext, cancel := context.WithTimeout(cmd.Context(), pipelineTimeout)
			defer cancel()
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

			mode, err := terraform.ParsePlanMode(planMode)
			if err != nil {
				return err
			}
			lockBackend, err := scanner.ParseLockBackend(lockBackendName)
			if err != nil {
				return err
			}
			scanOptions := scanner.Options{
				Directory:     directory,
				Timeout:       timeout,
				WorkspaceRoot: workspaceRoot,
				PlanMode:      mode,
				LockBackend:   lockBackend,
				SkipInit:      skipTerraformInit,
			}
			scanOptions, err = scanner.PrepareOptions(scanOptions)
			if err != nil {
				return err
			}
			if terraformExec {
				runner := terraform.NewCLIRunner(terraformBin)
				runner.Workspace = terraformWorkspace
				runner.VarFiles = append([]string(nil), varFiles...)
				runner.Vars = append([]string(nil), vars...)
				scanOptions.Runner = runner
				scanOptions.RequireTerraformFiles = true
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: bootstrap report only; pass --terraform-exec for a real drift scan")
			}

			result, err := scanner.Scan(scanContext, scanOptions)
			if err != nil {
				if redactPaths {
					return errors.New("scan failed")
				}
				return err
			}
			auditReport = result.Report

			scanReport := result.Report
			if err := report.ApplyIgnoreRules(&scanReport, append(append([]report.IgnoreRule(nil), baselineRules...), ignoreRules...)); err != nil {
				return err
			}
			report.ApplyOwners(&scanReport, resourceOwners)
			enrichedReport, err := enrichReport(scanContext, scanReport, costCommand, costArgs, auditCommand, auditArgs)
			if err != nil {
				return err
			}
			scanReport = enrichedReport
			if err := report.ApplyRunbooks(&scanReport, remediationRunbooks); err != nil {
				return err
			}
			if approvalFile != "" {
				data, err := ioutil.ReadLimitedFile(approvalFile, int64(maxApprovalBytes))
				if err != nil {
					return fmt.Errorf("read approval %s: %w", approvalFile, err)
				}
				var approval report.Approval
				if err := json.Unmarshal(data, &approval); err != nil {
					return fmt.Errorf("parse approval %s: %w", approvalFile, err)
				}
				if err := report.VerifyApproval(scanReport, approval); err != nil {
					return err
				}
				scanReport.Approval = &approval
			}
			if redactPaths {
				scanReport.Directory = "[REDACTED]"
			}
			if err := writeScanReport(stdout, scanReport, parsedFormat); err != nil {
				return err
			}
			if err := finalizeRootScan(scanContext, scanReport, deliveryOptions{
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
			}); err != nil {
				return err
			}
			if scanReport.Status == report.ScanStatusDriftDetected {
				if failureSeverity != "" {
					meets, err := report.MeetsSeverity(scanReport, failureSeverity)
					if err != nil {
						return err
					}
					if !meets {
						return nil
					}
				}
				return errDriftDetected
			}
			if scanReport.Status == report.ScanStatusChangesDetected {
				return errChangesDetected
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", ".", "Terraform directory to scan")
	cmd.Flags().StringVarP(&format, "output", "o", string(outputFormatTable), "output format: table, json, junit, sarif, prometheus")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "maximum scan duration")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact local filesystem paths from scan output")
	cmd.Flags().BoolVar(&terraformExec, "terraform-exec", false, "run Terraform init, plan, and show -json")
	cmd.Flags().StringVar(&terraformBin, "terraform-bin", "", "Terraform-compatible executable to run (default: terraform)")
	cmd.Flags().StringVar(&planMode, "plan-mode", string(terraform.PlanModeRefreshOnly), "plan mode: refresh-only (remote drift) or normal (configuration reconciliation)")
	cmd.Flags().StringVar(&scanConfigPath, "config", "", "optional TerraDrift config file to load")
	cmd.Flags().StringVar(&configProfile, "profile", "", "named config profile to load")
	cmd.Flags().StringVar(&failureSeverity, "failure-severity", "", "minimum drift severity that fails the scan: low, medium, high, critical")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "require the Terraform directory to resolve inside this workspace root")
	cmd.Flags().StringVar(&notifyTarget, "notify", "", "notification target: slack, teams, webhook")
	cmd.Flags().StringVar(&slackWebhookURL, "slack-webhook-url", "", "Slack incoming webhook URL")
	cmd.Flags().StringVar(&teamsWebhookURL, "teams-webhook-url", "", "Microsoft Teams incoming webhook URL")
	cmd.Flags().StringVar(&webhookURL, "webhook-url", "", "generic HTTPS webhook URL")
	cmd.Flags().StringVar(&webhookCACert, "webhook-ca-cert", "", "PEM CA certificate file for webhook TLS verification")
	cmd.Flags().StringVar(&githubRepository, "github-repository", "", "GitHub repository for pull request summary (owner/repo)")
	cmd.Flags().IntVar(&githubPR, "github-pr", 0, "GitHub pull request number for scan summary")
	cmd.Flags().IntVar(&githubIssueAfter, "github-issue-after", 0, "create a GitHub issue after this many consecutive matching drift scans")
	cmd.Flags().StringVar(&artifactURL, "artifact-url", "", "presigned HTTPS URL to upload the JSON report")
	cmd.Flags().StringVar(&approvalFile, "approval-file", "", "review-only approval artifact to attach to the report")
	cmd.Flags().StringVar(&auditCommand, "audit-command", "", "audit correlation command to enrich the scan report")
	cmd.Flags().StringArrayVar(&auditArgs, "audit-arg", nil, "audit command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&dashboardHTMLPath, "dashboard-html", "", "write a static HTML dashboard report to this path")
	cmd.Flags().StringVar(&historyDir, "history-dir", "", "write JSON scan history to this directory and include recent history in dashboards")
	cmd.Flags().IntVar(&historyRetention, "history-retention", 0, "maximum history reports to retain (0 keeps all)")
	cmd.Flags().BoolVar(&historyCompressed, "history-compressed", false, "store history reports as gzip-compressed JSON")
	cmd.Flags().StringVar(&auditLogPath, "audit-log", "", "append secret-safe JSON audit events to this path")
	cmd.Flags().StringVar(&policyCommand, "policy-command", "", "policy command to run with the scan report JSON on stdin")
	cmd.Flags().StringArrayVar(&policyArgs, "policy-arg", nil, "policy command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&costCommand, "cost-command", "", "cost command to enrich the scan report from JSON stdin/stdout")
	cmd.Flags().StringArrayVar(&costArgs, "cost-arg", nil, "cost command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&lockBackendName, "lock-backend", "local", "scan lock backend: local (single-host file lock)")
	cmd.Flags().BoolVar(&skipTerraformInit, "skip-terraform-init", false, "skip terraform init when .terraform is already valid")
	cmd.Flags().BoolVar(&attributeValues, "attribute-values", false, "include safe attribute values in history, artifacts, policy input, dashboards, and notifications (default: paths only)")
	cmd.Flags().StringVar(&terraformWorkspace, "workspace", "", "Terraform workspace to select before plan")
	cmd.Flags().StringArrayVar(&varFiles, "var-file", nil, "Terraform -var-file path; repeatable")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "Terraform -var assignment; repeatable")
	return cmd
}

func writeDashboard(path string, scanReport report.DriftReport, historyEntries []history.Entry) error {
	if err := rejectSymlink(path); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create dashboard HTML %s: %w", path, err)
	}
	if err := dashboard.RenderWithHistory(file, dashboard.Data{Current: scanReport, History: historyEntries}); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close dashboard HTML %s: %w", path, err)
	}
	return nil
}

func previousReportForRoot(entries []history.Entry, current report.DriftReport) report.DriftReport {
	for _, entry := range entries {
		if sameRoot(entry.Report, current) {
			return entry.Report
		}
	}
	return report.DriftReport{}
}

func shouldCreatePersistentIssue(current report.DriftReport, entries []history.Entry, threshold int) bool {
	if threshold < 2 || current.Status != report.ScanStatusDriftDetected || report.DriftFingerprint(current) == "" {
		return false
	}
	consecutive := 0
	for _, entry := range entries {
		previous := entry.Report
		if !sameRoot(previous, current) {
			continue
		}
		if previous.Status != report.ScanStatusDriftDetected || report.DriftFingerprint(previous) != report.DriftFingerprint(current) {
			break
		}
		consecutive++
	}
	return consecutive == threshold-1
}

func sameRoot(left, right report.DriftReport) bool {
	if left.RootID != "" && right.RootID != "" {
		return left.RootID == right.RootID
	}
	return left.Directory == right.Directory
}

func auditCommandNames(terraformExec bool, terraformBin, costCommand, policyCommand, auditCommand string) []string {
	commands := make([]string, 0, 4)
	if terraformExec {
		if terraformBin == "" {
			terraformBin = "terraform"
		}
		commands = append(commands, filepath.Base(terraformBin))
	}
	for _, command := range []string{costCommand, policyCommand, auditCommand} {
		if command != "" {
			commands = append(commands, filepath.Base(command))
		}
	}
	return commands
}
