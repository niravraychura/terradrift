package dashboard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestRenderEscapesResourceFields(t *testing.T) {
	var output bytes.Buffer
	err := RenderWithHistory(&output, Data{Current: report.DriftReport{
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
	if !strings.Contains(output.String(), "drift_detected") {
		t.Fatalf("expected status in dashboard, got %q", output.String())
	}
}
