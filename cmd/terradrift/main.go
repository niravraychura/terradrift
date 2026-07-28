package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
	cmd.AddCommand(newInitCommand(stdout))
	return cmd
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
			}

			parsedFormat, err := parseOutputFormat(format)
			if err != nil {
				return err
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
			if redactPaths {
				scanReport.Directory = "[REDACTED]"
			}
			if err := writeScanReport(stdout, scanReport, parsedFormat); err != nil {
				return err
			}
			var historyEntries []history.Entry
			if historyDir != "" {
				if _, err := history.Write(historyDir, scanReport); err != nil {
					return err
				}
				entries, err := history.LoadRecent(historyDir, 10)
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
			if notifyTarget != "" {
				if err := sendNotification(cmd.Context(), notifyTarget, slackWebhookURL, teamsWebhookURL, webhookURL, scanReport); err != nil {
					return err
				}
			}
			if result.Outcome == scanner.OutcomeDriftDetected {
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
	cmd.Flags().StringVar(&workspaceRoot, "workspace-root", "", "require the Terraform directory to resolve inside this workspace root")
	cmd.Flags().StringVar(&notifyTarget, "notify", "", "notification target: slack, teams, webhook")
	cmd.Flags().StringVar(&slackWebhookURL, "slack-webhook-url", "", "Slack incoming webhook URL")
	cmd.Flags().StringVar(&teamsWebhookURL, "teams-webhook-url", "", "Microsoft Teams incoming webhook URL")
	cmd.Flags().StringVar(&webhookURL, "webhook-url", "", "generic HTTPS webhook URL")
	cmd.Flags().StringVar(&dashboardHTMLPath, "dashboard-html", "", "write a static HTML dashboard report to this path")
	cmd.Flags().StringVar(&historyDir, "history-dir", "", "write JSON scan history to this directory and include recent history in dashboards")
	cmd.Flags().StringVar(&policyCommand, "policy-command", "", "policy command to run with the scan report JSON on stdin")
	cmd.Flags().StringArrayVar(&policyArgs, "policy-arg", nil, "policy command argument; repeat for multiple arguments")
	cmd.Flags().StringVar(&costCommand, "cost-command", "", "cost command to enrich the scan report from JSON stdin/stdout")
	cmd.Flags().StringArrayVar(&costArgs, "cost-arg", nil, "cost command argument; repeat for multiple arguments")
	return cmd
}

func writeDashboard(path string, scanReport report.DriftReport, historyEntries []history.Entry) error {
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
