package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePlanReturnsChangedResourcesAndTotals(t *testing.T) {
	plan := []byte(`{
		"resource_changes": [
			{"address":"aws_instance.web","type":"aws_instance","name":"web","provider_name":"registry.terraform.io/hashicorp/aws","change":{"actions":["update"]}},
			{"address":"aws_s3_bucket.logs","type":"aws_s3_bucket","name":"logs","change":{"actions":["no-op"]}},
			{"address":"aws_db_instance.db","type":"aws_db_instance","name":"db","change":{"actions":["delete","create"]}}
		]
	}`)

	changes, outputChanges, total, err := ParsePlan(plan)
	if err != nil {
		t.Fatalf("expected plan to parse: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total resources 3, got %d", total)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changed resources, got %d", len(changes))
	}
	if len(outputChanges) != 0 {
		t.Fatalf("expected no output changes, got %#v", outputChanges)
	}
	if changes[0].Address != "aws_db_instance.db" || len(changes[0].Actions) != 2 {
		t.Fatalf("unexpected first change: %#v", changes[0])
	}
	if changes[1].Address != "aws_instance.web" || changes[1].Actions[0] != "update" || changes[1].Remediation == "" || changes[1].ReconciliationHint == "" || changes[1].CloudProvider != "aws" {
		t.Fatalf("unexpected second change: %#v", changes[1])
	}
}

func TestParsePlanRejectsInvalidJSON(t *testing.T) {
	_, _, _, err := ParsePlan([]byte(`{"resource_changes":`))
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func BenchmarkParsePlanLargePlan(b *testing.B) {
	plan := largePlanFixture(1000)
	b.ResetTimer()
	for range b.N {
		if _, _, _, err := ParsePlan(plan); err != nil {
			b.Fatalf("parse large plan: %v", err)
		}
	}
}

func TestParsePlanRetainsSafeMetadataOnly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "safe_metadata_plan.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	changes, outputChanges, _, err := ParsePlan(data)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(changes) != 1 || changes[0].ActionReason != "replace_because_cannot_update" {
		t.Fatalf("expected action reason, got %#v", changes)
	}
	if len(outputChanges) != 1 || outputChanges[0].Name != "service_url" || outputChanges[0].Actions[0] != "update" {
		t.Fatalf("expected safe output metadata, got %#v", outputChanges)
	}
	encoded, err := json.Marshal(struct {
		Changes []interface{} `json:"changes"`
	}{Changes: []interface{}{changes, outputChanges}})
	if err != nil {
		t.Fatalf("marshal parsed report fields: %v", err)
	}
	if strings.Contains(string(encoded), "super-secret-value") {
		t.Fatalf("parsed report retained a Terraform value: %s", encoded)
	}
}

func largePlanFixture(resources int) []byte {
	var builder strings.Builder
	builder.WriteString(`{"resource_changes":[`)
	for i := range resources {
		if i > 0 {
			builder.WriteByte(',')
		}
		action := "no-op"
		if i%10 == 0 {
			action = "update"
		}
		_, _ = fmt.Fprintf(&builder, `{"address":"aws_instance.example_%d","type":"aws_instance","name":"example_%d","change":{"actions":["%s"]}}`, i, i, action)
	}
	builder.WriteString(`]}`)
	return []byte(builder.String())
}
