package dashboard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/report"
)

func TestRenderEscapesResourceFields(t *testing.T) {
	var output bytes.Buffer
	err := RenderWithHistory(&output, Data{Current: report.DriftReport{
		ScanID:                "scan-123",
		Status:                report.ScanStatusDriftDetected,
		TotalResourcesChecked: 1,
		TotalChangedResources: 1,
		ResourceChanges: []report.ResourceChange{{
			Address: `<script>alert("x")</script>`,
			Type:    "aws_instance",
			Name:    "web",
			Actions: []string{"update"},
		}},
	}})
	if err != nil {
		t.Fatalf("expected dashboard render to succeed: %v", err)
	}
	if strings.Contains(output.String(), "<script>") {
		t.Fatalf("expected HTML output to escape script tags, got %q", output.String())
	}
	if !strings.Contains(output.String(), "scan-123") || !strings.Contains(output.String(), "drift_detected") {
		t.Fatalf("expected status in dashboard, got %q", output.String())
	}
	if !strings.Contains(output.String(), "system-ui") {
		t.Fatalf("expected dashboard CSS, got %q", output.String())
	}
}

func TestRenderIndexEscapesDirectories(t *testing.T) {
	var output bytes.Buffer
	err := RenderIndex(&output, []history.Entry{{Report: report.DriftReport{Directory: `<script>alert("x")</script>`}}})
	if err != nil {
		t.Fatalf("expected dashboard index render to succeed: %v", err)
	}
	if strings.Contains(output.String(), "<script>") {
		t.Fatalf("expected index output to escape script tags, got %q", output.String())
	}
	if !strings.Contains(output.String(), "system-ui") {
		t.Fatalf("expected index CSS, got %q", output.String())
	}
}

func TestRenderIndexGroupsByDirectory(t *testing.T) {
	var output bytes.Buffer
	err := RenderIndex(&output, []history.Entry{
		{Report: report.DriftReport{Directory: "terraform/prod", ScanID: "scan-prod-1"}},
		{Report: report.DriftReport{Directory: "terraform/dev", ScanID: "scan-dev-1"}},
		{Report: report.DriftReport{Directory: "terraform/prod", ScanID: "scan-prod-2"}},
	})
	if err != nil {
		t.Fatalf("render dashboard index: %v", err)
	}
	got := output.String()
	prod := strings.Index(got, "terraform/prod")
	dev := strings.Index(got, "terraform/dev")
	if prod < 0 || dev < 0 || prod > dev {
		t.Fatalf("expected first-seen directory grouping, got %q", got)
	}
	if !strings.Contains(got, "scan-prod-1") || !strings.Contains(got, "scan-prod-2") || !strings.Contains(got, "scan-dev-1") {
		t.Fatalf("expected all scan IDs, got %q", got)
	}
}

func TestRenderIndexEmpty(t *testing.T) {
	var output bytes.Buffer
	if err := RenderIndex(&output, nil); err != nil {
		t.Fatalf("render dashboard index: %v", err)
	}
	if !strings.Contains(output.String(), "No history available") {
		t.Fatalf("expected empty index message, got %q", output.String())
	}
}

func TestRenderWithHistoryIncludesTrend(t *testing.T) {
	var output bytes.Buffer
	err := RenderWithHistory(&output, Data{History: []history.Entry{{Report: report.DriftReport{Status: report.ScanStatusDriftDetected}}, {Report: report.DriftReport{Status: report.ScanStatusFailed}}}})
	if err != nil {
		t.Fatalf("render dashboard: %v", err)
	}
	if !strings.Contains(output.String(), "1 drifted, 0 with configuration changes, and 1 failed scans across 2 recent scans") {
		t.Fatalf("expected trend summary, got %q", output.String())
	}
}
