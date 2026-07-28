package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

var errDriftDetected = errors.New("drift detected")

const (
	exitCodeOK            = 0
	exitCodeFailure       = 1
	exitCodeDriftDetected = 2
)

type outputFormat string

const (
	outputFormatTable outputFormat = "table"
	outputFormatJSON  outputFormat = "json"
	outputFormatJUnit outputFormat = "junit"
)

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		code := exitCodeForError(err)
		if code != exitCodeDriftDetected {
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
	cmd.AddCommand(newInitCommand(stdout))
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

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Validate a Terraform directory for drift scanning",
		Example: `  terradrift scan
  terradrift scan --directory ./terraform/prod
  terradrift scan -d ./terraform/prod --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if scanConfigPath != "" {
				cfg, err := config.Load(scanConfigPath)
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
	cmd.Flags().StringVarP(&format, "output", "o", string(outputFormatTable), "output format: table, json, junit")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "maximum scan duration")
	cmd.Flags().BoolVar(&redactPaths, "redact-paths", false, "redact local filesystem paths from scan output")
	cmd.Flags().BoolVar(&terraformExec, "terraform-exec", false, "run Terraform init, refresh-only plan, and show -json")
	cmd.Flags().StringVar(&terraformBin, "terraform-bin", "", "Terraform-compatible executable to run (default: terraform)")
	cmd.Flags().StringVar(&scanConfigPath, "config", "", "optional TerraDrift config file to load")
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
	case outputFormatTable, outputFormatJSON, outputFormatJUnit:
		return outputFormat(normalized), nil
	default:
		return "", fmt.Errorf("unsupported output format %q; supported values: table, json, junit", format)
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
		return fmt.Errorf("unsupported output format %q; supported values: table, json, junit", format)
	}
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
