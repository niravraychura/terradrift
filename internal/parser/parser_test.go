package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/terraform"
)

func TestParsePlanReturnsChangedResourcesAndTotals(t *testing.T) {
	plan := []byte(`{
		"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"},{"mode":"managed"},{"mode":"managed"}]}}},
		"resource_changes": [
			{"address":"aws_instance.web","type":"aws_instance","name":"web","provider_name":"registry.terraform.io/hashicorp/aws","change":{"actions":["update"]}},
			{"address":"aws_s3_bucket.logs","type":"aws_s3_bucket","name":"logs","change":{"actions":["no-op"]}},
			{"address":"aws_db_instance.db","type":"aws_db_instance","name":"db","change":{"actions":["delete","create"]}}
		]
	}`)

	changes, outputChanges, total, exact, err := ParsePlan(plan, terraform.PlanModeNormal)
	if err != nil {
		t.Fatalf("expected plan to parse: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total resources 3, got %d", total)
	}
	if !exact {
		t.Fatal("expected prior-state count to be exact")
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
	_, _, _, _, err := ParsePlan([]byte(`{"resource_changes":`), terraform.PlanModeRefreshOnly)
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func BenchmarkParsePlanLargePlan(b *testing.B) {
	plan := largePlanFixture(1000)
	b.ResetTimer()
	for range b.N {
		if _, _, _, _, err := ParsePlan(plan, terraform.PlanModeNormal); err != nil {
			b.Fatalf("parse large plan: %v", err)
		}
	}
}

func TestParsePlanDropsTerraformValuesAndSensitiveMarks(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "safe_metadata_plan.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	changes, outputChanges, _, _, err := ParsePlan(data, terraform.PlanModeNormal)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(changes) != 1 || changes[0].ActionReason != "replace_because_cannot_update" {
		t.Fatalf("expected action reason, got %#v", changes)
	}
	if len(outputChanges) != 1 || outputChanges[0].Name != "service_url" || outputChanges[0].Actions[0] != "update" {
		t.Fatalf("expected safe output metadata, got %#v", outputChanges)
	}
	attrs := map[string]report.AttributeChange{}
	for _, attr := range changes[0].AttributeChanges {
		attrs[attr.Path] = attr
	}
	if got := attrs["instance_type"]; got.Before != `"t3.micro"` || got.After != `"t3.small"` {
		t.Fatalf("expected instance_type diff, got %#v", got)
	}
	if got := attrs["tags.Env"]; got.Before != `"old"` || got.After != `"new"` {
		t.Fatalf("expected tags.Env diff, got %#v", got)
	}
	if got := attrs["password"]; got.Before != "[REDACTED]" || got.After != "[REDACTED]" {
		t.Fatalf("expected redacted password diff, got %#v", got)
	}
	encoded, err := json.Marshal(struct {
		Changes []interface{} `json:"changes"`
	}{Changes: []interface{}{changes, outputChanges}})
	if err != nil {
		t.Fatalf("marshal parsed report fields: %v", err)
	}
	for _, forbidden := range []string{"super-secret-value", "another-super-secret", "before_sensitive", "after_sensitive"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("parsed report retained %q: %s", forbidden, encoded)
		}
	}
}

func TestParsePlanModeAndPriorStateSemantics(t *testing.T) {
	tests := []struct {
		name       string
		mode       terraform.PlanMode
		plan       string
		wantChange []string
		wantTotal  int
		wantExact  bool
	}{
		{
			name:       "refresh only reads resource drift",
			mode:       terraform.PlanModeRefreshOnly,
			plan:       `{"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"},{"mode":"managed"}]} }},"resource_drift":[{"address":"aws_instance.remote","mode":"managed","change":{"actions":["update"]}}],"resource_changes":[{"address":"aws_instance.config","mode":"managed","change":{"actions":["create"]}}]}`,
			wantChange: []string{"aws_instance.remote"}, wantTotal: 2, wantExact: true,
		},
		{
			name:       "refresh fallback supports older JSON",
			mode:       terraform.PlanModeRefreshOnly,
			plan:       `{"prior_state":{"values":{"root_module":{"resources":[]}}},"resource_changes":[{"address":"aws_instance.remote","mode":"managed","change":{"actions":["update"]}}]}`,
			wantChange: []string{"aws_instance.remote"}, wantExact: true,
		},
		{
			name:       "normal ignores resource drift",
			mode:       terraform.PlanModeNormal,
			plan:       `{"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"}]}}},"resource_drift":[{"address":"aws_instance.remote","mode":"managed","change":{"actions":["update"]}}],"resource_changes":[{"address":"aws_instance.config","mode":"managed","change":{"actions":["update"]}}]}`,
			wantChange: []string{"aws_instance.config"}, wantTotal: 1, wantExact: true,
		},
		{
			name:      "nested count and for each instances",
			mode:      terraform.PlanModeRefreshOnly,
			plan:      `{"prior_state":{"values":{"root_module":{"resources":[{"address":"aws_instance.counted[0]","mode":"managed"},{"address":"data.aws_region.current","mode":"data"}],"child_modules":[{"resources":[{"address":"module.child.aws_instance.counted[1]","mode":"managed"},{"address":"module.child.aws_instance.each[\"blue\"]","mode":"managed"}],"child_modules":[{"resources":[{"address":"module.child.module.nested.aws_instance.each[\"green\"]","mode":"managed"}]}]}]}}},"resource_drift":[]}`,
			wantTotal: 4, wantExact: true,
		},
		{
			name:       "no op data read and imports",
			mode:       terraform.PlanModeNormal,
			plan:       `{"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"}]}}},"resource_changes":[{"address":"aws_instance.noop","mode":"managed","change":{"actions":["no-op"]}},{"address":"data.aws_region.current","mode":"data","change":{"actions":["read"]}},{"address":"aws_instance.read","mode":"managed","change":{"actions":["read"]}},{"address":"aws_instance.imported","mode":"managed","action_reason":"import","change":{"actions":["create"]}}]}`,
			wantChange: []string{"aws_instance.imported"}, wantTotal: 1, wantExact: true,
		},
		{
			name:      "missing prior state is estimated",
			mode:      terraform.PlanModeRefreshOnly,
			plan:      `{"resource_drift":[],"resource_changes":[{"address":"aws_instance.one","mode":"managed","change":{"actions":["no-op"]}},{"address":"data.aws_region.current","mode":"data","change":{"actions":["read"]}}]}`,
			wantTotal: 1, wantExact: false,
		},
		{
			name:       "null prior state is estimated",
			mode:       terraform.PlanModeNormal,
			plan:       `{"prior_state":null,"resource_changes":[{"address":"aws_instance.one","mode":"managed","change":{"actions":["update"]}}]}`,
			wantChange: []string{"aws_instance.one"}, wantTotal: 1, wantExact: false,
		},
		{
			name:      "null optional module sections are safe",
			mode:      terraform.PlanModeRefreshOnly,
			plan:      `{"prior_state":{"values":{"root_module":{"resources":null,"child_modules":null}}},"resource_drift":[]}`,
			wantTotal: 0, wantExact: true,
		},
		{
			name:      "empty prior state is exact",
			mode:      terraform.PlanModeRefreshOnly,
			plan:      `{"prior_state":{"values":{"root_module":{"resources":[]}}},"resource_drift":[]}`,
			wantTotal: 0, wantExact: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changes, _, total, exact, err := ParsePlan([]byte(test.plan), test.mode)
			if err != nil || total != test.wantTotal || exact != test.wantExact {
				t.Fatalf("ParsePlan() = changes=%#v total=%d exact=%t err=%v", changes, total, exact, err)
			}
			addresses := make([]string, len(changes))
			for i := range changes {
				addresses[i] = changes[i].Address
			}
			if got, want := strings.Join(addresses, ","), strings.Join(test.wantChange, ","); got != want {
				t.Fatalf("changes = %q, want %q", got, want)
			}
		})
	}
}

func TestParsePlanRejectsMalformedOptionalChangeSection(t *testing.T) {
	_, _, _, _, err := ParsePlan([]byte(`{"prior_state":{"values":{"root_module":{"resources":[]}}},"resource_drift":{}}`), terraform.PlanModeRefreshOnly)
	if err == nil {
		t.Fatal("expected malformed resource_drift to fail safely")
	}
}

func TestCountPriorStateIgnoresResourceValues(t *testing.T) {
	huge := strings.Repeat("x", 10000)
	plan := []byte(`{
		"prior_state":{"values":{"root_module":{"resources":[
			{"mode":"managed","values":{"blob":` + mustQuoteJSON(huge) + `}},
			{"mode":"data","values":{"blob":` + mustQuoteJSON(huge) + `}},
			{"mode":"managed","values":{"blob":` + mustQuoteJSON(huge) + `}}
		]}}},
		"resource_changes":[]
	}`)
	_, _, total, exact, err := ParsePlan(plan, terraform.PlanModeRefreshOnly)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if !exact || total != 2 {
		t.Fatalf("expected exact managed count 2, got total=%d exact=%t", total, exact)
	}
}

func TestParsePlanSkipsUnusedTopLevelFields(t *testing.T) {
	huge := strings.Repeat("y", 20000)
	plan := []byte(`{
		"format_version":"1.2",
		"configuration":{"root_module":{"blob":` + mustQuoteJSON(huge) + `}},
		"planned_values":{"root_module":{"resources":[{"values":{"blob":` + mustQuoteJSON(huge) + `}}]}},
		"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"},{"mode":"managed"}]}}},
		"resource_drift":[{"address":"aws_instance.web","type":"aws_instance","name":"web","mode":"managed","change":{"actions":["update"],"before":{"idle_timeout":60},"after":{"idle_timeout":120}}}],
		"resource_changes":[{"address":"aws_instance.other","mode":"managed","change":{"actions":["create"]}}]
	}`)
	changes, _, total, exact, err := ParsePlan(plan, terraform.PlanModeRefreshOnly)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if !exact || total != 2 {
		t.Fatalf("expected exact count 2, got total=%d exact=%t", total, exact)
	}
	if len(changes) != 1 || changes[0].Address != "aws_instance.web" {
		t.Fatalf("expected drift-only change, got %#v", changes)
	}
	if len(changes[0].AttributeChanges) == 0 {
		t.Fatal("expected attribute diffs for drifted resource")
	}
}

func TestParsePlanReaderMatchesParsePlan(t *testing.T) {
	plan := []byte(`{"prior_state":{"values":{"root_module":{"resources":[{"mode":"managed"}]}}},"resource_changes":[{"address":"aws_instance.web","type":"aws_instance","name":"web","mode":"managed","change":{"actions":["update"]}}]}`)
	aChanges, aOutputs, aTotal, aExact, aErr := ParsePlan(plan, terraform.PlanModeNormal)
	bChanges, bOutputs, bTotal, bExact, bErr := ParsePlanReader(bytes.NewReader(plan), terraform.PlanModeNormal)
	if aErr != nil || bErr != nil {
		t.Fatalf("parse errors: %v %v", aErr, bErr)
	}
	if aTotal != bTotal || aExact != bExact || len(aChanges) != len(bChanges) || len(aOutputs) != len(bOutputs) {
		t.Fatalf("mismatch ParsePlan vs Reader: %#v vs totals %d/%d exact %t/%t", aChanges, aTotal, bTotal, aExact, bExact)
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
