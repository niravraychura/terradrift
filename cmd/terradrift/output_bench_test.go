package main

import (
	"io"
	"testing"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

func BenchmarkWriteScanReportJSON(b *testing.B) {
	scanReport := largeBenchmarkReport()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeScanReport(io.Discard, scanReport, outputFormatJSON); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteScanReportTable(b *testing.B) {
	scanReport := largeBenchmarkReport()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeScanReport(io.Discard, scanReport, outputFormatTable); err != nil {
			b.Fatal(err)
		}
	}
}

func largeBenchmarkReport() report.DriftReport {
	changes := make([]report.ResourceChange, 0, 200)
	for i := 0; i < 200; i++ {
		changes = append(changes, report.ResourceChange{
			Address:   "aws_instance.web",
			Type:      "aws_instance",
			Name:      "web",
			Actions:   []string{"update"},
			RiskLevel: "medium",
			AttributeChanges: []report.AttributeChange{{
				Path:   "instance_type",
				Before: "t3.micro",
				After:  "t3.small",
			}},
		})
	}
	now := time.Unix(0, 0).UTC()
	return report.DriftReport{
		ScanID:                "00000000-0000-4000-8000-000000000001",
		Status:                report.ScanStatusDriftDetected,
		Directory:             "/workspace/terraform/prod",
		PlanMode:              "refresh-only",
		ResourceChanges:       changes,
		TotalResourcesChecked: 500,
		TotalChangedResources: len(changes),
		StartedAt:             now,
		CompletedAt:           now.Add(time.Second),
	}
}
