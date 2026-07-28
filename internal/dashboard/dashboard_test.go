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
}
