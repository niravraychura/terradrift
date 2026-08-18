package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWithoutAttributeValuesClearsBeforeAfter(t *testing.T) {
	secret := "super-secret-password-value"
	input := DriftReport{
		Status: ScanStatusDriftDetected,
		ResourceChanges: []ResourceChange{{
			Address: "aws_db_instance.main",
			AttributeChanges: []AttributeChange{
				{Path: "password", Before: secret, After: "[REDACTED]"},
				{Path: "idle_timeout", Before: "60", After: "120"},
			},
			Actions: []string{"update"},
		}},
		ProviderVersions: map[string]string{"aws": "5.0.0"},
	}
	out := WithoutAttributeValues(input)
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "60") || strings.Contains(string(encoded), "120") {
		t.Fatalf("secret or values leaked in paths-only report: %s", encoded)
	}
	if len(out.ResourceChanges[0].AttributeChanges) != 2 {
		t.Fatalf("expected paths preserved, got %#v", out.ResourceChanges[0].AttributeChanges)
	}
	for _, attr := range out.ResourceChanges[0].AttributeChanges {
		if attr.Before != "" || attr.After != "" {
			t.Fatalf("expected empty before/after, got %#v", attr)
		}
		if attr.Path == "" {
			t.Fatal("expected path to remain")
		}
	}
	if input.ResourceChanges[0].AttributeChanges[0].Before != secret {
		t.Fatal("WithoutAttributeValues mutated the input report")
	}
}
