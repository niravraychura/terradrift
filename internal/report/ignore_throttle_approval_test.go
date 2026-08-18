package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestApplyIgnoreRulesClearsDriftWhenAllIgnored(t *testing.T) {
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	scanReport := DriftReport{
		Status: ScanStatusDriftDetected,
		ResourceChanges: []ResourceChange{
			{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"},
		},
		TotalChangedResources: 1,
	}
	if err := ApplyIgnoreRules(&scanReport, []IgnoreRule{{
		Address: "aws_instance.web", Owner: "platform", Reason: "expected", ExpiresAt: expires,
	}}); err != nil {
		t.Fatalf("ApplyIgnoreRules: %v", err)
	}
	if scanReport.Status != ScanStatusNoDrift || scanReport.TotalChangedResources != 0 {
		t.Fatalf("unexpected report after ignore: %#v", scanReport)
	}
	if !scanReport.ResourceChanges[0].Ignored || scanReport.ResourceChanges[0].IgnoreOwner != "platform" {
		t.Fatalf("ignore annotation missing: %#v", scanReport.ResourceChanges[0])
	}
}

func TestApplyIgnoreRulesRejectsExpired(t *testing.T) {
	expires := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	err := ApplyIgnoreRules(&DriftReport{}, []IgnoreRule{{
		Address: "aws_instance.web", Owner: "platform", Reason: "stale", ExpiresAt: expires,
	}})
	if err == nil {
		t.Fatal("expected expired ignore rule to fail")
	}
}

func TestApplyOwnersPrefersExactAddress(t *testing.T) {
	scanReport := DriftReport{ResourceChanges: []ResourceChange{
		{Address: "aws_instance.web", Type: "aws_instance"},
		{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket"},
	}}
	ApplyOwners(&scanReport, map[string]string{
		"aws_instance.web": "alice",
		"aws_s3_bucket":    "storage-team",
	})
	if scanReport.ResourceChanges[0].Owner != "alice" {
		t.Fatalf("exact owner = %q", scanReport.ResourceChanges[0].Owner)
	}
	if scanReport.ResourceChanges[1].Owner != "storage-team" {
		t.Fatalf("type owner = %q", scanReport.ResourceChanges[1].Owner)
	}
}

func TestShouldNotifyOnEscalation(t *testing.T) {
	previous := DriftReport{
		Status: ScanStatusDriftDetected,
		ResourceChanges: []ResourceChange{
			{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"},
		},
	}
	current := DriftReport{
		Status: ScanStatusDriftDetected,
		ResourceChanges: []ResourceChange{
			{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "high"},
		},
	}
	if !ShouldNotify(current, previous) {
		t.Fatal("expected notify on risk escalation")
	}
	if ShouldNotify(previous, previous) {
		t.Fatal("expected throttle for unchanged drift")
	}
}

func TestApprovalRoundTrip(t *testing.T) {
	scanReport := DriftReport{
		Status: ScanStatusDriftDetected,
		ResourceChanges: []ResourceChange{
			{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"},
		},
	}
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	approval, err := NewApproval(scanReport, "alice", "reviewed", expires)
	if err != nil {
		t.Fatalf("NewApproval: %v", err)
	}
	if err := VerifyApproval(scanReport, approval); err != nil {
		t.Fatalf("VerifyApproval: %v", err)
	}
	scanReport.ResourceChanges[0].RiskLevel = "high"
	if err := VerifyApproval(scanReport, approval); err == nil {
		t.Fatal("expected fingerprint mismatch")
	}
}

func TestWithoutAttributeValuesStripsSecrets(t *testing.T) {
	secret := "fixture-redaction-probe-v1"
	scanReport := DriftReport{
		Status: ScanStatusDriftDetected,
		ResourceChanges: []ResourceChange{{
			Address: "aws_db_instance.main",
			AttributeChanges: []AttributeChange{
				{Path: "password", Before: secret, After: secret},
				{Path: "idle_timeout", Before: "60", After: "120"},
			},
		}},
	}
	persisted := WithoutAttributeValues(scanReport)
	data, err := json.Marshal(persisted)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), secret) || strings.Contains(string(data), "120") {
		t.Fatalf("attribute values leaked into paths-only report: %s", data)
	}
	if !strings.Contains(string(data), `"path":"password"`) {
		t.Fatalf("expected password path to remain: %s", data)
	}
}
