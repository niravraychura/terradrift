package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/niravraychura/terradrift/internal/report"
)

type outputFormat string

const (
	outputFormatTable      outputFormat = "table"
	outputFormatJSON       outputFormat = "json"
	outputFormatJUnit      outputFormat = "junit"
	outputFormatSARIF      outputFormat = "sarif"
	outputFormatPrometheus outputFormat = "prometheus"
)

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
		if report.HasChanges(scanReport.Status) {
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
		ruleID, ruleName, messagePrefix := "terradrift.drift", "Terraform drift detected", "Terraform drift"
		if scanReport.Status == report.ScanStatusChangesDetected {
			ruleID, ruleName, messagePrefix = "terradrift.change", "Terraform changes detected", "Terraform configuration change"
		}
		results := make([]sarifResult, 0, len(scanReport.ResourceChanges))
		for _, change := range scanReport.ResourceChanges {
			if change.Ignored {
				continue
			}
			results = append(results, sarifResult{RuleID: ruleID, Level: "error", Message: sarifMessage{Text: fmt.Sprintf("%s: %s", messagePrefix, change.Address)}})
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(sarifLog{
			Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
			Version: "2.1.0",
			Runs: []sarifRun{{
				Tool:    sarifTool{Driver: sarifDriver{Name: "TerraDrift", Rules: []sarifRule{{ID: ruleID, Name: ruleName}}}},
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
			"# HELP terradrift_resources_changed Resources changed by the scan.",
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
		if _, err := fmt.Fprintf(stdout, "Plan mode: %s\n", scanReport.PlanMode); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		if _, err := fmt.Fprintf(stdout, "Scan ID: %s\n", scanReport.ScanID); err != nil {
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
		if len(scanReport.ResourceChanges) == 0 {
			return nil
		}
		if _, err := fmt.Fprintln(stdout); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		for _, change := range scanReport.ResourceChanges {
			if change.Ignored {
				continue
			}
			actions := strings.Join(change.Actions, ",")
			risk := strings.ToUpper(change.RiskLevel)
			if risk == "" {
				risk = "UNKNOWN"
			}
			if _, err := fmt.Fprintf(stdout, "%s  %s  %s\n", risk, actions, change.Address); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
			if change.ActionReason != "" {
				if _, err := fmt.Fprintf(stdout, "  reason: %s\n", change.ActionReason); err != nil {
					return fmt.Errorf("write scan output: %w", err)
				}
			}
			for _, attr := range change.AttributeChanges {
				if _, err := fmt.Fprintf(stdout, "  %s: %s -> %s\n", attr.Path, attr.Before, attr.After); err != nil {
					return fmt.Errorf("write scan output: %w", err)
				}
			}
		}
		if len(scanReport.OutputChanges) > 0 {
			if _, err := fmt.Fprintln(stdout); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
			if _, err := fmt.Fprintln(stdout, "Output changes:"); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
			for _, outputChange := range scanReport.OutputChanges {
				if _, err := fmt.Fprintf(stdout, "  %s: %s\n", outputChange.Name, strings.Join(outputChange.Actions, ",")); err != nil {
					return fmt.Errorf("write scan output: %w", err)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q; supported values: table, json, junit, sarif, prometheus", format)
	}
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
		fmt.Sprintf("Status: %s", aggregate.Status),
		fmt.Sprintf("Roots scanned: %d", aggregate.TotalRoots),
		fmt.Sprintf("Drifted roots: %d", aggregate.DriftedRoots),
		fmt.Sprintf("Changed roots: %d", aggregate.ChangedRoots),
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
