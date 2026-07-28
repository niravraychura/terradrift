package report

import (
	"strings"
	"testing"
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
