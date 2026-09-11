package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestRedactedNotificationMessageIncludesDetailsWithoutValues(t *testing.T) {
	message := RedactedNotificationMessage(report.DriftReport{
		ScanID:                "scan-1",
		Status:                report.ScanStatusDriftDetected,
		PlanMode:              "refresh-only",
		Directory:             "/secret/local/path",
		TotalResourcesChecked: 150,
		TotalChangedResources: 2,
		StartedAt:             time.Date(2026, 9, 11, 8, 45, 11, 0, time.UTC),
		CompletedAt:           time.Date(2026, 9, 11, 8, 45, 54, 0, time.UTC),
		ResourceChanges: []report.ResourceChange{
			{
				Address:   "module.alb.aws_lb.main",
				Type:      "aws_lb",
				Actions:   []string{"update"},
				RiskLevel: "medium",
				AttributeChanges: []report.AttributeChange{
					{Path: "idle_timeout"},
					{Path: `tags["aws-apn-id"]`},
				},
			},
			{
				Address:   "aws_instance.web",
				Type:      "aws_instance",
				Actions:   []string{"delete"},
				RiskLevel: "high",
				AttributeChanges: []report.AttributeChange{
					{Path: "ami"},
				},
			},
			{
				Address:   "aws_s3_bucket.ignored",
				Type:      "aws_s3_bucket",
				Actions:   []string{"update"},
				RiskLevel: "medium",
				Ignored:   true,
			},
		},
	})
	if strings.Contains(message, "/secret/local/path") {
		t.Fatalf("notification leaked directory: %s", message)
	}
	for _, want := range []string{
		"Resources checked: 150",
		"Changed resources: 2",
		"Duration: 43s",
		"By risk: high 1, medium 1",
		"HIGH  delete  aws_instance  aws_instance.web",
		"MEDIUM  update  aws_lb  module.alb.aws_lb.main",
		"  idle_timeout",
		`  tags["aws-apn-id"]`,
		"  ami",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in notification, got %q", want, message)
		}
	}
	if strings.Contains(message, "aws_s3_bucket.ignored") {
		t.Fatalf("ignored resource leaked into notification: %s", message)
	}
	if strings.Contains(message, "Attribute sides:") {
		t.Fatalf("paths-only notification should omit value-side legend, got %q", message)
	}
	if idxHigh, idxMed := strings.Index(message, "HIGH  delete"), strings.Index(message, "MEDIUM  update"); idxHigh < 0 || idxMed < 0 || idxHigh > idxMed {
		t.Fatalf("expected high-risk finding before medium, got %q", message)
	}
}

func TestRedactedNotificationMessageIncludesValuesWhenPresent(t *testing.T) {
	message := RedactedNotificationMessage(report.DriftReport{
		PlanMode:              "refresh-only",
		TotalResourcesChecked: 10,
		TotalChangedResources: 1,
		ResourceChanges: []report.ResourceChange{{
			Address:   "module.alb.aws_lb.main",
			Type:      "aws_lb",
			Actions:   []string{"update"},
			RiskLevel: "medium",
			AttributeChanges: []report.AttributeChange{
				{Path: "idle_timeout", Before: "120", After: "600"},
				{Path: "password", Before: "[REDACTED]", After: "[REDACTED]"},
			},
		}},
	})
	for _, want := range []string{
		"Attribute sides: state -> remote",
		"idle_timeout: 120 -> 600",
		"password: [REDACTED] -> [REDACTED]",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in notification, got %q", want, message)
		}
	}
}

func TestRedactedNotificationMessageCapsFindingsAndAttributes(t *testing.T) {
	changes := make([]report.ResourceChange, 0, maxNotifyChanges+2)
	for i := 0; i < maxNotifyChanges+2; i++ {
		attrs := make([]report.AttributeChange, maxNotifyAttrs+1)
		for j := range attrs {
			attrs[j] = report.AttributeChange{Path: "attr"}
		}
		changes = append(changes, report.ResourceChange{
			Address:          "aws_lb.n" + string(rune('a'+i)),
			Type:             "aws_lb",
			Actions:          []string{"update"},
			RiskLevel:        "medium",
			AttributeChanges: attrs,
		})
	}
	message := RedactedNotificationMessage(report.DriftReport{
		TotalChangedResources: len(changes),
		ResourceChanges:       changes,
	})
	if !strings.Contains(message, "… and 2 more") {
		t.Fatalf("expected resource cap remainder, got %q", message)
	}
	if !strings.Contains(message, "… and 1 more attributes") {
		t.Fatalf("expected attribute cap remainder, got %q", message)
	}
}
