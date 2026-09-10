package report

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStableScanReportJSONKeys(t *testing.T) {
	data, err := json.Marshal(DriftReport{
		ScanID:                "scan-1",
		Status:                ScanStatusNoDrift,
		Directory:             "terraform/prod",
		PlanMode:              "refresh-only",
		TotalResourcesChecked: 1,
		ResourcesCheckedExact: true,
		TotalChangedResources: 0,
		ResourceChanges:       []ResourceChange{},
		StartedAt:             time.Unix(0, 0).UTC(),
		CompletedAt:           time.Unix(1, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"scan_id", "status", "directory", "plan_mode",
		"total_resources_checked", "resources_checked_exact", "total_changed_resources",
		"resource_changes", "started_at", "completed_at",
	} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing stable field %q in %s", key, data)
		}
	}
}
