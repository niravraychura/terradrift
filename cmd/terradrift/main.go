package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/niravraychura/terradrift/internal/audit"
	"github.com/niravraychura/terradrift/internal/command"
	"github.com/niravraychura/terradrift/internal/config"
	"github.com/niravraychura/terradrift/internal/cost"
	"github.com/niravraychura/terradrift/internal/dashboard"
	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/notify"
	"github.com/niravraychura/terradrift/internal/policy"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
	"github.com/niravraychura/terradrift/internal/terraform"
	"github.com/spf13/cobra"
)

var (
	errDriftDetected   = errors.New("drift detected")
	errMultiScanFailed = errors.New("one or more scans failed")
)

const (
	exitCodeOK            = 0
	exitCodeFailure       = 1
	exitCodeDriftDetected = 2
)

type outputFormat string

const (
	outputFormatTable      outputFormat = "table"
	outputFormatJSON       outputFormat = "json"
	outputFormatJUnit      outputFormat = "junit"
	outputFormatSARIF      outputFormat = "sarif"
	outputFormatPrometheus outputFormat = "prometheus"
)

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		code := exitCodeForError(err)
		if code != exitCodeDriftDetected && !errors.Is(err, errMultiScanFailed) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(code)
	}
	os.Exit(exitCodeOK)
}

func exitCodeForError(err error) int {
	if errors.Is(err, errDriftDetected) {
		return exitCodeDriftDetected
	}
	return exitCodeFailure
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "terradrift",
		Short:         "Self-hosted Terraform drift detection",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newScanCommand(stdout))
	cmd.AddCommand(newScanAllCommand(stdout))
	cmd.AddCommand(newDashboardIndexCommand(stdout))
	cmd.AddCommand(newServeCommand(stdout))
	cmd.AddCommand(newApproveCommand(stdout))
	cmd.AddCommand(newInitCommand(stdout))
	return cmd
}

func newApproveCommand(stdout io.Writer) *cobra.Command {
	var reportPath string
	var owner string
	var reason string
	var expiresAt string
	var output string
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Create a review-only approval for a drift report",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(reportPath)
			if err != nil {
				return fmt.Errorf("read report %s: %w", reportPath, err)
			}
			var scanReport report.DriftReport
			if err := json.Unmarshal(data, &scanReport); err != nil {
				return fmt.Errorf("parse report %s: %w", reportPath, err)
			}
			approval, err := report.NewApproval(scanReport, owner, reason, expiresAt)
			if err != nil {
				return err
			}
			if output == "" {
				output = reportPath + ".approval.json"
			}
			data, err = json.MarshalIndent(approval, "", "  ")
			if err != nil {
				return fmt.Errorf("encode approval: %w", err)
			}
			data = append(data, '\n')
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return fmt.Errorf("create approval %s: %w", output, err)
			}
			if _, err := file.Write(data); err != nil {
				_ = file.Close()
				return fmt.Errorf("write approval %s: %w", output, err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close approval %s: %w", output, err)
			}
			_, err = fmt.Fprintf(stdout, "Created approval: %s\n", output)
			return err
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "JSON drift report to approve")
	cmd.Flags().StringVar(&owner, "owner", "", "approver identity")
	cmd.Flags().StringVar(&reason, "reason", "", "approval reason")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "approval expiry in RFC3339 format")
	cmd.Flags().StringVar(&output, "output", "", "approval artifact path")
	for _, name := range []string{"report", "owner", "reason", "expires-at"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(err)
		}
	}
	return cmd
}

func newDashboardIndexCommand(stdout io.Writer) *cobra.Command {
	var historyDir string
	var output string
	var limit int
	cmd := &cobra.Command{
		Use:   "dashboard-index",
		Short: "Write a static dashboard index from scan history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				return fmt.Errorf("limit must be greater than zero")
			}
			entries, err := history.LoadRecent(historyDir, limit)
			if err != nil {
				return err
			}
			if err := rejectSymlink(output); err != nil {
				return err
			}
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("create dashboard index %s: %w", output, err)
			}
			if err := dashboard.RenderIndex(file, entries); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close dashboard index %s: %w", output, err)
			}
			_, err = fmt.Fprintf(stdout, "Wrote dashboard index: %s\n", output)
			return err
		},
	}
	cmd.Flags().StringVar(&historyDir, "history-dir", ".terradrift-history", "directory containing scan history")
	cmd.Flags().StringVar(&output, "output", "terradrift-index.html", "HTML dashboard index path")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum history reports to include")
	return cmd
}

func newServeCommand(stdout io.Writer) *cobra.Command {
	var historyDir string
	var listen string
	var limit int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve local scan history over a read-only API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLocalListenAddress(listen); err != nil {
				return err
			}
			if limit <= 0 {
				return fmt.Errorf("limit must be greater than zero")
			}
			listener, err := net.Listen("tcp", listen)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", listen, err)
			}
			server := &http.Server{Handler: newHistoryHandler(historyDir, limit), ReadHeaderTimeout: 5 * time.Second}
			go func() {
				<-cmd.Context().Done()
				_ = server.Shutdown(context.Background())
			}()
			if _, err := fmt.Fprintf(stdout, "Serving scan history at http://%s\n", listener.Addr()); err != nil {
				_ = listener.Close()
				return fmt.Errorf("write server address: %w", err)
			}
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve scan history: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&historyDir, "history-dir", ".terradrift-history", "directory containing scan history")
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:8080", "loopback address to listen on")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum reports to serve")
	return cmd
}

func validateLocalListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen address must be loopback-only")
	}
	return nil
}

func newHistoryHandler(historyDir string, limit int) http.Handler {
	load := func() ([]history.Entry, error) {
		return history.LoadRecent(historyDir, limit)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/reports", func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		entries, err := load()
		if err != nil {
			http.Error(w, "load scan history", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(entries); err != nil {
			return
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if request.URL.Path != "/" {
			http.NotFound(w, request)
			return
		}
		entries, err := load()
		if err != nil {
			http.Error(w, "load scan history", http.StatusInternalServerError)
			return
		}
		data := dashboard.Data{History: entries}
		if len(entries) > 0 {
			data.Current = entries[0].Report
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := dashboard.RenderWithHistory(w, data); err != nil {
			return
		}
	})
	return mux
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

	cmd := &cobra.Command{
		Use:   "scan-all",
		Short: "Scan Terraform roots from a manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (manifest == "") == (discover == "") {
				return fmt.Errorf("provide exactly one of --manifest or --discover")
			}
			var directories []string
			var err error
			if manifest != "" {
				directories, err = loadScanManifest(manifest)
			} else {
				directories, err = discoverTerraformRoots(discover, includes, excludes)
			}
			if err != nil {
				return err
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

			options := scanner.Options{Timeout: timeout, WorkspaceRoot: workspaceRoot}
			if terraformExec {
				options.Runner = terraform.NewCLIRunner(terraformBin)
			}
			aggregate := scanAll(cmd.Context(), directories, options, concurrency, redactPaths)
			if err := writeMultiScanReport(stdout, aggregate, parsedFormat); err != nil {
				return err
			}
			if aggregate.FailedRoots > 0 {
				return fmt.Errorf("%w: %d roots", errMultiScanFailed, aggregate.FailedRoots)
			}
			if aggregate.DriftedRoots > 0 {
				return errDriftDetected
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&manifest, "manifest", "", "newline-delimited Terraform root manifest")
	cmd.Flags().StringVar(&discover, "discover", "", "workspace root to discover Terraform roots")
	cmd.Flags().StringArrayVar(&includes, "include", nil, "root-relative include pattern; repeatable")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "root-relative exclude pattern; repeatable")
	cmd.Flags().StringVarP(&format, "output", "o", string(outputFormatTable), "output format: table, json")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "maximum scan duration per root")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "maximum concurrent scans")
	cmd.Flags().BoolVar(&terraformExec, "terraform-exec", false, "run Terraform-compatible scans")
	cmd.Flags().StringVar(&terraformBin, "terraform-bin", "", "Terraform-compatible executable to run (default: terraform)")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "require roots to resolve inside this workspace root")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact local filesystem paths from scan output")
	return cmd
}

func newInitCommand(stdout io.Writer) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter TerraDrift config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.WriteDefault(path); err != nil {
				return err
			}
			_, err := fmt.Fprintf(stdout, "Created TerraDrift config: %s\n", path)
			return err
		},
	}
	cmd.Flags().StringVar(&path, "config", config.DefaultPath, "config file path to create")
	return cmd
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
	var dashboardHTMLPath string
	var historyDir string
	var policyCommand string
	var policyArgs []string
	var costCommand string
	var costArgs []string
	var remediationRunbooks map[string]string
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

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Validate a Terraform directory for drift scanning",
		Example: `  terradrift scan
  terradrift scan --directory ./terraform/prod
  terradrift scan -d ./terraform/prod --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if scanConfigPath != "" || configProfile != "" {
				cfg, err := config.LoadProfile(scanConfigPath, configProfile)
				if err != nil {
					return err
				}
				if !cmd.Flags().Changed("directory") {
					directory = cfg.Directory
				}
				if !cmd.Flags().Changed("output") {
					format = cfg.Output
				}
				if !cmd.Flags().Changed("timeout") {
					parsedTimeout, err := time.ParseDuration(cfg.Timeout)
					if err != nil {
						return fmt.Errorf("parse config timeout: %w", err)
					}
					timeout = parsedTimeout
				}
				if !cmd.Flags().Changed("redact-paths") {
					redactPaths = cfg.RedactPaths
				}
				if !cmd.Flags().Changed("terraform-exec") {
					terraformExec = cfg.TerraformExec
				}
				if !cmd.Flags().Changed("terraform-bin") {
					terraformBin = cfg.TerraformBin
				}
				if !cmd.Flags().Changed("workspace-root") {
					workspaceRoot = cfg.WorkspaceRoot
				}
				if !cmd.Flags().Changed("notify") {
					notifyTarget = cfg.Notify
				}
				if !cmd.Flags().Changed("slack-webhook-url") {
					slackWebhookURL = cfg.SlackWebhookURL
				}
				if !cmd.Flags().Changed("teams-webhook-url") {
					teamsWebhookURL = cfg.TeamsWebhookURL
				}
				if !cmd.Flags().Changed("webhook-url") {
					webhookURL = cfg.WebhookURL
				}
				if !cmd.Flags().Changed("dashboard-html") {
					dashboardHTMLPath = cfg.DashboardHTML
				}
				if !cmd.Flags().Changed("history-dir") {
					historyDir = cfg.HistoryDir
				}
				if !cmd.Flags().Changed("policy-command") {
					policyCommand = cfg.PolicyCommand
				}
				if !cmd.Flags().Changed("policy-arg") {
					policyArgs = append([]string(nil), cfg.PolicyArgs...)
				}
				if !cmd.Flags().Changed("cost-command") {
					costCommand = cfg.CostCommand
				}
				if !cmd.Flags().Changed("cost-arg") {
					costArgs = append([]string(nil), cfg.CostArgs...)
				}
				remediationRunbooks = cfg.RemediationRunbooks
				ignoreRules = cfg.IgnoreRules
				resourceOwners = cfg.ResourceOwners
				ownerWebhooks = cfg.OwnerWebhooks
				notificationThrottle = cfg.NotificationThrottle
				if !cmd.Flags().Changed("github-repository") {
					githubRepository = cfg.GitHubRepository
				}
				if !cmd.Flags().Changed("github-pr") {
					githubPR = cfg.GitHubPR
				}
				if !cmd.Flags().Changed("github-issue-after") {
					githubIssueAfter = cfg.GitHubIssueAfter
				}
				if !cmd.Flags().Changed("artifact-url") {
					artifactURL = cfg.ArtifactURL
				}
				if !cmd.Flags().Changed("audit-command") {
					auditCommand = cfg.AuditCommand
				}
				if !cmd.Flags().Changed("audit-arg") {
					auditArgs = append([]string(nil), cfg.AuditArgs...)
				}
				allowedCommands = append([]string(nil), cfg.AllowedCommands...)
				trustedCommandDirs = append([]string(nil), cfg.TrustedCommandDirs...)
				if !cmd.Flags().Changed("failure-severity") {
					failureSeverity = cfg.FailureSeverity
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

			scanOptions := scanner.Options{
				Directory:     directory,
				Timeout:       timeout,
				WorkspaceRoot: workspaceRoot,
			}
			if terraformExec {
				scanOptions.Runner = terraform.NewCLIRunner(terraformBin)
			}

			result, err := scanner.Scan(cmd.Context(), scanOptions)
			if err != nil {
				return err
			}

			scanReport := result.Report
			if err := report.ApplyIgnoreRules(&scanReport, ignoreRules); err != nil {
				return err
			}
			report.ApplyOwners(&scanReport, resourceOwners)
			if costCommand != "" {
				enrichedReport, err := cost.Enrich(cmd.Context(), cost.Options{Command: costCommand, Args: costArgs}, scanReport)
				if err != nil {
					return err
				}
				scanReport = enrichedReport
			}
			if err := report.ApplyRunbooks(&scanReport, remediationRunbooks); err != nil {
				return err
			}
			if auditCommand != "" {
				enrichedReport, err := audit.Enrich(cmd.Context(), audit.Options{Command: auditCommand, Args: auditArgs}, scanReport)
				if err != nil {
					return err
				}
				scanReport = enrichedReport
			}
			if approvalFile != "" {
				data, err := os.ReadFile(approvalFile)
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
			if artifactURL != "" {
				artifact, err := json.Marshal(scanReport)
				if err != nil {
					return fmt.Errorf("encode report artifact: %w", err)
				}
				if err := (notify.ArtifactUploader{URL: artifactURL}).Upload(cmd.Context(), artifact, "application/json"); err != nil {
					return err
				}
			}
			var historyEntries []history.Entry
			var previousReport report.DriftReport
			if historyDir != "" {
				entries, err := history.LoadRecent(historyDir, 100)
				if err != nil {
					return err
				}
				previousReport = previousReportForDirectory(entries, scanReport.Directory)
				if shouldCreatePersistentIssue(scanReport, entries, githubIssueAfter) {
					if err := (notify.GitHubIssueNotifier{Repository: githubRepository, Token: os.Getenv("GITHUB_TOKEN")}).Notify(cmd.Context(), scanReport); err != nil {
						return err
					}
				}
				if _, err := history.Write(historyDir, scanReport); err != nil {
					return err
				}
				entries, err = history.LoadRecent(historyDir, 10)
				if err != nil {
					return err
				}
				historyEntries = entries
			}
			if dashboardHTMLPath != "" {
				if err := writeDashboard(dashboardHTMLPath, scanReport, historyEntries); err != nil {
					return err
				}
			}
			if policyCommand != "" {
				if err := policy.Run(cmd.Context(), policy.Options{Command: policyCommand, Args: policyArgs}, scanReport); err != nil {
					return err
				}
			}
			shouldNotify := !notificationThrottle || report.ShouldNotify(scanReport, previousReport)
			if notifyTarget != "" && shouldNotify {
				if err := sendNotification(cmd.Context(), notifyTarget, slackWebhookURL, teamsWebhookURL, webhookURL, scanReport); err != nil {
					return err
				}
			}
			if githubRepository != "" && githubPR > 0 && shouldNotify {
				if err := (notify.GitHubPRNotifier{Repository: githubRepository, Number: githubPR, Token: os.Getenv("GITHUB_TOKEN")}).Notify(cmd.Context(), scanReport); err != nil {
					return err
				}
			}
			for owner, webhookURL := range ownerWebhooks {
				ownerReport := scanReport
				ownerReport.ResourceChanges = nil
				for _, change := range scanReport.ResourceChanges {
					if change.Owner == owner && !change.Ignored {
						ownerReport.ResourceChanges = append(ownerReport.ResourceChanges, change)
					}
				}
				if len(ownerReport.ResourceChanges) == 0 {
					continue
				}
				ownerReport.TotalChangedResources = len(ownerReport.ResourceChanges)
				if notificationThrottle {
					previousOwnerReport := previousReport
					previousOwnerReport.ResourceChanges = nil
					for _, change := range previousReport.ResourceChanges {
						if change.Owner == owner && !change.Ignored {
							previousOwnerReport.ResourceChanges = append(previousOwnerReport.ResourceChanges, change)
						}
					}
					previousOwnerReport.TotalChangedResources = len(previousOwnerReport.ResourceChanges)
					if !report.ShouldNotify(ownerReport, previousOwnerReport) {
						continue
					}
				}
				if err := (notify.WebhookNotifier{WebhookURL: webhookURL}).Notify(cmd.Context(), ownerReport); err != nil {
					return err
				}
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
			return nil
		},
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", ".", "Terraform directory to scan")
	cmd.Flags().StringVarP(&format, "output", "o", string(outputFormatTable), "output format: table, json, junit, sarif, prometheus")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "maximum scan duration")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact local filesystem paths from scan output")
	cmd.Flags().BoolVar(&terraformExec, "terraform-exec", false, "run Terraform init, refresh-only plan, and show -json")
	cmd.Flags().StringVar(&terraformBin, "terraform-bin", "", "Terraform-compatible executable to run (default: terraform)")
	cmd.Flags().StringVar(&scanConfigPath, "config", "", "optional TerraDrift config file to load")
	cmd.Flags().StringVar(&configProfile, "profile", "", "named config profile to load")
	cmd.Flags().StringVar(&failureSeverity, "failure-severity", "", "minimum drift severity that fails the scan: low, medium, high, critical")
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "require the Terraform directory to resolve inside this workspace root")
	cmd.Flags().StringVar(&notifyTarget, "notify", "", "notification target: slack, teams, webhook")
	cmd.Flags().StringVar(&slackWebhookURL, "slack-webhook-url", "", "Slack incoming webhook URL")
	cmd.Flags().StringVar(&teamsWebhookURL, "teams-webhook-url", "", "Microsoft Teams incoming webhook URL")
	cmd.Flags().StringVar(&webhookURL, "webhook-url", "", "generic HTTPS webhook URL")
	cmd.Flags().StringVar(&githubRepository, "github-repository", "", "GitHub repository for pull request summary (owner/repo)")
	cmd.Flags().IntVar(&githubPR, "github-pr", 0, "GitHub pull request number for scan summary")
	cmd.Flags().IntVar(&githubIssueAfter, "github-issue-after", 0, "create a GitHub issue after this many consecutive matching drift scans")
	cmd.Flags().StringVar(&artifactURL, "artifact-url", "", "presigned HTTPS URL to upload the JSON report")
	cmd.Flags().StringVar(&approvalFile, "approval-file", "", "review-only approval artifact to attach to the report")
	cmd.Flags().StringVar(&auditCommand, "audit-command", "", "audit correlation command to enrich the scan report")
	cmd.Flags().StringArrayVar(&auditArgs, "audit-arg", nil, "audit command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&dashboardHTMLPath, "dashboard-html", "", "write a static HTML dashboard report to this path")
	cmd.Flags().StringVar(&historyDir, "history-dir", "", "write JSON scan history to this directory and include recent history in dashboards")
	cmd.Flags().StringVar(&policyCommand, "policy-command", "", "policy command to run with the scan report JSON on stdin")
	cmd.Flags().StringArrayVar(&policyArgs, "policy-arg", nil, "policy command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&costCommand, "cost-command", "", "cost command to enrich the scan report from JSON stdin/stdout")
	cmd.Flags().StringArrayVar(&costArgs, "cost-arg", nil, "cost command argument; repeat for multiple arguments")
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

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output path must not be a symlink: %s", path)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect output path %s: %w", path, err)
	}
	return nil
}

func previousReportForDirectory(entries []history.Entry, directory string) report.DriftReport {
	for _, entry := range entries {
		if entry.Report.Directory == directory {
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
		if previous.Directory != current.Directory {
			continue
		}
		if previous.Status != report.ScanStatusDriftDetected || report.DriftFingerprint(previous) != report.DriftFingerprint(current) {
			break
		}
		consecutive++
	}
	return consecutive == threshold-1
}

func sendNotification(ctx context.Context, target string, slackWebhookURL string, teamsWebhookURL string, webhookURL string, scanReport report.DriftReport) error {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "slack":
		return notify.SlackNotifier{WebhookURL: slackWebhookURL}.Notify(ctx, scanReport)
	case "teams":
		return notify.TeamsNotifier{WebhookURL: teamsWebhookURL}.Notify(ctx, scanReport)
	case "webhook":
		return notify.WebhookNotifier{WebhookURL: webhookURL}.Notify(ctx, scanReport)
	default:
		return fmt.Errorf("unsupported notification target %q; supported values: slack, teams, webhook", target)
	}
}

func parseOutputFormat(format string) (outputFormat, error) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	switch outputFormat(normalized) {
	case outputFormatTable, outputFormatJSON, outputFormatJUnit, outputFormatSARIF, outputFormatPrometheus:
		return outputFormat(normalized), nil
	default:
		return "", fmt.Errorf("unsupported output format %q; supported values: table, json, junit, sarif, prometheus", format)
	}
}

func writeScanReport(stdout io.Writer, scanReport report.DriftReport, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(scanReport); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	case outputFormatJUnit:
		suite := junitTestSuite{Name: "terradrift", Tests: 1, TestCases: []junitTestCase{{Name: "scan", ClassName: "terradrift"}}}
		if scanReport.Status == report.ScanStatusDriftDetected {
			suite.Failures = 1
			suite.TestCases[0].Failure = &junitFailure{Message: fmt.Sprintf("%d resources changed", scanReport.TotalChangedResources)}
		}
		if _, err := io.WriteString(stdout, xml.Header); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if err := xml.NewEncoder(stdout).Encode(junitTestSuites{Suites: []junitTestSuite{suite}}); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	case outputFormatSARIF:
		results := make([]sarifResult, 0, len(scanReport.ResourceChanges))
		for _, change := range scanReport.ResourceChanges {
			results = append(results, sarifResult{RuleID: "terradrift.drift", Level: "error", Message: sarifMessage{Text: fmt.Sprintf("Terraform drift: %s", change.Address)}})
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(sarifLog{
			Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
			Version: "2.1.0",
			Runs: []sarifRun{{
				Tool:    sarifTool{Driver: sarifDriver{Name: "TerraDrift", Rules: []sarifRule{{ID: "terradrift.drift", Name: "Terraform drift detected"}}}},
				Results: results,
			}},
		}); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	case outputFormatPrometheus:
		duration := scanReport.CompletedAt.Sub(scanReport.StartedAt).Seconds()
		failures := 0
		if scanReport.Status == report.ScanStatusFailed {
			failures = 1
		}
		for _, line := range []string{
			"# HELP terradrift_scan_status Scan result status.",
			"# TYPE terradrift_scan_status gauge",
			fmt.Sprintf("terradrift_scan_status{status=%q} 1", scanReport.Status),
			"# HELP terradrift_scan_duration_seconds Scan duration in seconds.",
			"# TYPE terradrift_scan_duration_seconds gauge",
			fmt.Sprintf("terradrift_scan_duration_seconds %g", duration),
			"# HELP terradrift_resources_checked Resources checked by the scan.",
			"# TYPE terradrift_resources_checked gauge",
			fmt.Sprintf("terradrift_resources_checked %d", scanReport.TotalResourcesChecked),
			"# HELP terradrift_resources_changed Resources with detected drift.",
			"# TYPE terradrift_resources_changed gauge",
			fmt.Sprintf("terradrift_resources_changed %d", scanReport.TotalChangedResources),
			"# HELP terradrift_scan_failures Failed scans.",
			"# TYPE terradrift_scan_failures gauge",
			fmt.Sprintf("terradrift_scan_failures %d", failures),
		} {
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
		}
		return nil
	case outputFormatTable:
		if _, err := fmt.Fprintln(stdout, "TerraDrift scan initialized"); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Status: %s\n", scanReport.Status); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Terraform directory: %s\n", scanReport.Directory); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Resources checked: %d\n", scanReport.TotalResourcesChecked); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Changed resources: %d\n", scanReport.TotalChangedResources); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; supported values: table, json, junit, sarif, prometheus", format)
	}
}

type multiScanReport struct {
	Roots                 []multiScanRoot `json:"roots"`
	TotalRoots            int             `json:"total_roots"`
	DriftedRoots          int             `json:"drifted_roots"`
	FailedRoots           int             `json:"failed_roots"`
	TotalResourcesChecked int             `json:"total_resources_checked"`
	TotalChangedResources int             `json:"total_changed_resources"`
}

type multiScanRoot struct {
	Directory string             `json:"directory"`
	Report    report.DriftReport `json:"report,omitempty"`
	Error     string             `json:"error,omitempty"`
}

func loadScanManifest(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan manifest %s: %w", path, err)
	}
	base := filepath.Dir(path)
	directories := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		directory := strings.TrimSpace(line)
		if directory == "" || strings.HasPrefix(directory, "#") {
			continue
		}
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(base, directory)
		}
		directories = append(directories, directory)
	}
	if len(directories) == 0 {
		return nil, fmt.Errorf("scan manifest %s has no Terraform roots", path)
	}
	return directories, nil
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

func scanAll(ctx context.Context, directories []string, options scanner.Options, concurrency int, redactPaths bool) multiScanReport {
	roots := make([]multiScanRoot, len(directories))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(concurrency, len(directories)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				root := multiScanRoot{Directory: directories[index]}
				rootOptions := options
				rootOptions.Directory = directories[index]
				result, err := scanner.Scan(ctx, rootOptions)
				if err != nil {
					if redactPaths {
						root.Directory = "[REDACTED]"
						root.Error = "scan failed"
					} else {
						root.Error = err.Error()
					}
				} else {
					root.Report = result.Report
					if redactPaths {
						root.Directory = "[REDACTED]"
						root.Report.Directory = "[REDACTED]"
					}
				}
				roots[index] = root
			}
		}()
	}
	for index := range directories {
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
		if root.Report.Status == report.ScanStatusDriftDetected {
			aggregate.DriftedRoots++
		}
	}
	return aggregate
}

func writeMultiScanReport(stdout io.Writer, aggregate multiScanReport, format outputFormat) error {
	if format == outputFormatJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(aggregate); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	}
	for _, line := range []string{
		"TerraDrift multi-root scan complete",
		fmt.Sprintf("Roots scanned: %d", aggregate.TotalRoots),
		fmt.Sprintf("Drifted roots: %d", aggregate.DriftedRoots),
		fmt.Sprintf("Failed roots: %d", aggregate.FailedRoots),
		fmt.Sprintf("Resources checked: %d", aggregate.TotalResourcesChecked),
		fmt.Sprintf("Changed resources: %d", aggregate.TotalChangedResources),
	} {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
	}
	return nil
}

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type sarifResult struct {
	RuleID  string       `json:"ruleId"`
	Level   string       `json:"level"`
	Message sarifMessage `json:"message"`
}

type sarifMessage struct {
	Text string `json:"text"`
}
