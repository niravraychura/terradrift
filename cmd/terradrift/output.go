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
		return writePrometheusScan(stdout, scanReport)
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
	switch format {
	case outputFormatPrometheus:
		return writePrometheusMultiScan(stdout, aggregate)
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(aggregate); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
		return nil
	case outputFormatJUnit:
		return writeMultiScanJUnit(stdout, aggregate)
	case outputFormatSARIF:
		return writeMultiScanSARIF(stdout, aggregate)
	case outputFormatTable:
		return writeMultiScanTable(stdout, aggregate)
	default:
		return fmt.Errorf("unsupported output format %q; supported values: table, json, junit, sarif, prometheus", format)
	}
}

func writeMultiScanTable(stdout io.Writer, aggregate multiScanReport) error {
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
	for _, root := range aggregate.Roots {
		switch {
		case root.Error != "":
			if _, err := fmt.Fprintf(stdout, "FAILED  %s  %s\n", root.Directory, root.Error); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
		case report.HasChanges(root.Report.Status):
			if _, err := fmt.Fprintf(stdout, "%s  %s  changed=%d\n", strings.ToUpper(string(root.Report.Status)), root.Directory, root.Report.TotalChangedResources); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
		}
	}
	return nil
}

func writeMultiScanJUnit(stdout io.Writer, aggregate multiScanReport) error {
	suite := junitTestSuite{Name: "terradrift", Tests: len(aggregate.Roots), TestCases: make([]junitTestCase, 0, len(aggregate.Roots))}
	for _, root := range aggregate.Roots {
		name := root.Directory
		if name == "" {
			name = "root"
		}
		testCase := junitTestCase{Name: name, ClassName: "terradrift"}
		switch {
		case root.Error != "":
			suite.Failures++
			testCase.Failure = &junitFailure{Message: root.Error}
		case report.HasChanges(root.Report.Status):
			suite.Failures++
			testCase.Failure = &junitFailure{Message: fmt.Sprintf("%d resources changed", root.Report.TotalChangedResources)}
		}
		suite.TestCases = append(suite.TestCases, testCase)
	}
	if _, err := io.WriteString(stdout, xml.Header); err != nil {
		return fmt.Errorf("write scan output: %w", err)
	}
	if err := xml.NewEncoder(stdout).Encode(junitTestSuites{Suites: []junitTestSuite{suite}}); err != nil {
		return fmt.Errorf("write scan output: %w", err)
	}
	return nil
}

func writeMultiScanSARIF(stdout io.Writer, aggregate multiScanReport) error {
	results := make([]sarifResult, 0)
	for _, root := range aggregate.Roots {
		if root.Error != "" {
			results = append(results, sarifResult{RuleID: "terradrift.failed", Level: "error", Message: sarifMessage{Text: fmt.Sprintf("%s: %s", root.Directory, root.Error)}})
			continue
		}
		ruleID, prefix := "terradrift.drift", "Terraform drift"
		if root.Report.Status == report.ScanStatusChangesDetected {
			ruleID, prefix = "terradrift.change", "Terraform configuration change"
		}
		for _, change := range root.Report.ResourceChanges {
			if change.Ignored {
				continue
			}
			results = append(results, sarifResult{RuleID: ruleID, Level: "error", Message: sarifMessage{Text: fmt.Sprintf("%s: %s %s", prefix, root.Directory, change.Address)}})
		}
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{Name: "TerraDrift", Rules: []sarifRule{
				{ID: "terradrift.drift", Name: "Terraform drift detected"},
				{ID: "terradrift.change", Name: "Terraform changes detected"},
				{ID: "terradrift.failed", Name: "Terraform scan failed"},
			}}},
			Results: results,
		}},
	}); err != nil {
		return fmt.Errorf("write scan output: %w", err)
	}
	return nil
}

func prometheusRootID(id string) string {
	if id == "" {
		return "default"
	}
	return id
}

func writePrometheusScan(stdout io.Writer, scanReport report.DriftReport) error {
	for _, line := range prometheusScanHelp() {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
	}
	return writePrometheusScanSamples(stdout, scanReport)
}

func prometheusScanHelp() []string {
	return []string{
		"# HELP terradrift_scan_status Scan result status. root_id is a bounded hash per Terraform root, never a filesystem path.",
		"# TYPE terradrift_scan_status gauge",
		"# HELP terradrift_scan_duration_seconds Scan duration in seconds.",
		"# TYPE terradrift_scan_duration_seconds gauge",
		"# HELP terradrift_resources_checked Resources checked by the scan.",
		"# TYPE terradrift_resources_checked gauge",
		"# HELP terradrift_resources_changed Resources changed by the scan.",
		"# TYPE terradrift_resources_changed gauge",
		"# HELP terradrift_scan_failures Failed scans.",
		"# TYPE terradrift_scan_failures gauge",
	}
}

func writePrometheusScanSamples(stdout io.Writer, scanReport report.DriftReport) error {
	rootID := prometheusRootID(scanReport.RootID)
	duration := scanReport.CompletedAt.Sub(scanReport.StartedAt).Seconds()
	failures := 0
	if scanReport.Status == report.ScanStatusFailed {
		failures = 1
	}
	for _, line := range []string{
		fmt.Sprintf("terradrift_scan_status{status=%q,root_id=%q} 1", scanReport.Status, rootID),
		fmt.Sprintf("terradrift_scan_duration_seconds{root_id=%q} %g", rootID, duration),
		fmt.Sprintf("terradrift_resources_checked{root_id=%q} %d", rootID, scanReport.TotalResourcesChecked),
		fmt.Sprintf("terradrift_resources_changed{root_id=%q} %d", rootID, scanReport.TotalChangedResources),
		fmt.Sprintf("terradrift_scan_failures{root_id=%q} %d", rootID, failures),
	} {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
	}
	return nil
}

func writePrometheusMultiScan(stdout io.Writer, aggregate multiScanReport) error {
	for _, line := range []string{
		"# HELP terradrift_roots Multi-root scan counts. result is a bounded enum.",
		"# TYPE terradrift_roots gauge",
		fmt.Sprintf("terradrift_roots{result=%q} %d", "total", aggregate.TotalRoots),
		fmt.Sprintf("terradrift_roots{result=%q} %d", "drifted", aggregate.DriftedRoots),
		fmt.Sprintf("terradrift_roots{result=%q} %d", "changed", aggregate.ChangedRoots),
		fmt.Sprintf("terradrift_roots{result=%q} %d", "failed", aggregate.FailedRoots),
	} {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
	}
	for _, line := range prometheusScanHelp() {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return fmt.Errorf("write scan output: %w", err)
		}
	}
	for _, root := range aggregate.Roots {
		if root.Error != "" {
			continue
		}
		if err := writePrometheusScanSamples(stdout, root.Report); err != nil {
			return err
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
