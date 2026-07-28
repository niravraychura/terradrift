package report

import (
	"strings"
	"testing"
	"time"
)

func TestRemediationForActions(t *testing.T) {
	cases := map[string][]string{
		"update drift":      {"update"},
		"replacement drift": {"delete", "create"},
		"deletion drift":    {"delete"},
		"missing resource":  {"create"},
	}
	for want, actions := range cases {
		t.Run(want, func(t *testing.T) {
			got := strings.ToLower(RemediationForActions(actions))
			if !strings.Contains(got, "review") || !strings.Contains(got, "approval") {
				t.Fatalf("expected reviewed remediation guidance, got %q", got)
			}
		})
	}
}

func TestApplyRunbooks(t *testing.T) {
	scanReport := DriftReport{ResourceChanges: []ResourceChange{{Type: "aws_instance", Actions: []string{"update"}}, {Type: "aws_s3_bucket", Actions: []string{"delete"}}}}
	if err := ApplyRunbooks(&scanReport, map[string]string{"aws_instance": "https://example.com/instance", "aws_s3_bucket/delete": "https://example.com/bucket-delete"}); err != nil {
		t.Fatalf("apply runbooks: %v", err)
	}
	if scanReport.ResourceChanges[0].RunbookURL != "https://example.com/instance" || scanReport.ResourceChanges[1].RunbookURL != "https://example.com/bucket-delete" {
		t.Fatalf("unexpected runbooks: %#v", scanReport.ResourceChanges)
	}
}

func TestApplyRunbooksRejectsUnsafeURL(t *testing.T) {
	err := ApplyRunbooks(&DriftReport{ResourceChanges: []ResourceChange{{Type: "aws_instance"}}}, map[string]string{"aws_instance": "http://example.com/runbook"})
	if err == nil {
		t.Fatal("expected unsafe runbook URL to be rejected")
	}
}

func TestReconciliationHintForActions(t *testing.T) {
	for _, actions := range [][]string{{"create"}, {"delete"}, {"delete", "create"}, {"update"}} {
		if hint := strings.ToLower(ReconciliationHintForActions(actions)); !strings.Contains(hint, "review only") {
			t.Fatalf("expected review-only hint for %v, got %q", actions, hint)
		}
	}
}

func TestApplyIgnoreRules(t *testing.T) {
	scanReport := DriftReport{Status: ScanStatusDriftDetected, TotalChangedResources: 1, ResourceChanges: []ResourceChange{{Address: "aws_instance.web"}}}
	rules := []IgnoreRule{{Address: "aws_instance.web", Owner: "platform", Reason: "approved maintenance", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}
	if err := ApplyIgnoreRules(&scanReport, rules); err != nil {
		t.Fatalf("apply ignore rules: %v", err)
	}
	if scanReport.Status != ScanStatusNoDrift || scanReport.TotalChangedResources != 0 || !scanReport.ResourceChanges[0].Ignored || scanReport.ResourceChanges[0].IgnoreOwner != "platform" {
		t.Fatalf("unexpected ignored report: %#v", scanReport)
	}
}

func TestApplyIgnoreRulesRejectsExpiredRule(t *testing.T) {
	err := ApplyIgnoreRules(&DriftReport{}, []IgnoreRule{{Address: "aws_instance.web", Owner: "platform", Reason: "expired", ExpiresAt: "2020-01-01T00:00:00Z"}})
	if err == nil {
		t.Fatal("expected expired rule to be rejected")
	}
}

func TestRiskLevelAndSeverity(t *testing.T) {
	if RiskLevelForActions([]string{"delete", "create"}) != "critical" {
		t.Fatal("expected replacement to be critical")
	}
	scanReport := DriftReport{ResourceChanges: []ResourceChange{{RiskLevel: "medium"}, {RiskLevel: "high", Ignored: true}}}
	if meets, err := MeetsSeverity(scanReport, "high"); err != nil || meets {
		t.Fatalf("expected ignored high risk to not meet gate: %v, %v", meets, err)
	}
}
