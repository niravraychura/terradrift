package cost

import (
	"context"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestEnrichAddsCostImpactByAddress(t *testing.T) {
	scanReport := report.DriftReport{ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web"}}}
	enriched, err := Enrich(context.Background(), Options{Command: "sh", Args: []string{"-c", `cat >/dev/null; printf '{"resource_costs":[{"address":"aws_instance.web","monthly_delta":"+$12.34/mo"}]}'`}}, scanReport)
	if err != nil {
		t.Fatalf("expected cost enrichment to pass: %v", err)
	}
	if got := enriched.ResourceChanges[0].CostImpact; got != "+$12.34/mo" {
		t.Fatalf("expected cost impact, got %q", got)
	}
}

func TestEnrichRedactsCostFailures(t *testing.T) {
	_, err := Enrich(context.Background(), Options{Command: "sh", Args: []string{"-c", "echo 'password=secret-value' >&2; exit 1"}}, report.DriftReport{})
	if err == nil {
		t.Fatal("expected cost command failure")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected cost error to be redacted, got %v", err)
	}
}

func TestEnrichRejectsInvalidJSON(t *testing.T) {
	_, err := Enrich(context.Background(), Options{Command: "printf", Args: []string{"not-json"}}, report.DriftReport{})
	if err == nil || !strings.Contains(err.Error(), "parse cost command output") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}
