package parser

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/terraform"
)

type terraformPlan struct {
	ResourceChanges json.RawMessage            `json:"resource_changes"`
	ResourceDrift   json.RawMessage            `json:"resource_drift"`
	OutputChanges   map[string]terraformChange `json:"output_changes"`
	PriorState      *terraformState            `json:"prior_state"`
}

type terraformState struct {
	Values *terraformStateValues `json:"values"`
}

type terraformStateValues struct {
	RootModule *terraformModule `json:"root_module"`
}

type terraformModule struct {
	Resources    []terraformStateResource `json:"resources"`
	ChildModules []terraformModule        `json:"child_modules"`
}

type terraformStateResource struct {
	Mode string `json:"mode"`
}

type terraformResourceChange struct {
	Address      string          `json:"address"`
	Type         string          `json:"type"`
	Name         string          `json:"name"`
	Mode         string          `json:"mode"`
	ProviderName string          `json:"provider_name"`
	ActionReason string          `json:"action_reason"`
	Change       terraformChange `json:"change"`
}

type terraformChange struct {
	// Values are decoded only into redacted attribute diffs; raw secrets never enter reports.
	Actions         []string        `json:"actions"`
	Before          json.RawMessage `json:"before"`
	After           json.RawMessage `json:"after"`
	AfterUnknown    json.RawMessage `json:"after_unknown"`
	BeforeSensitive json.RawMessage `json:"before_sensitive"`
	AfterSensitive  json.RawMessage `json:"after_sensitive"`
}

// ParsePlan converts the subset of Terraform plan JSON needed for reports.
//
// Refresh-only plans use resource_drift when it is present. Older Terraform and
// OpenTofu JSON renderers can omit that field, so only then do we fall back to
// resource_changes. Prior-state inventory is exact when its root module exists;
// otherwise resource_changes supplies a clearly marked estimate.
func ParsePlan(data []byte, mode terraform.PlanMode) ([]report.ResourceChange, []report.OutputChange, int, bool, error) {
	mode, err := terraform.ParsePlanMode(string(mode))
	if err != nil {
		return nil, nil, 0, false, err
	}
	var plan terraformPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, nil, 0, false, fmt.Errorf("parse terraform plan JSON: %w", err)
	}

	resourceChanges, err := decodeResourceChanges(plan.ResourceChanges)
	if err != nil {
		return nil, nil, 0, false, err
	}
	selected := resourceChanges
	if mode == terraform.PlanModeRefreshOnly && plan.ResourceDrift != nil {
		selected, err = decodeResourceChanges(plan.ResourceDrift)
		if err != nil {
			return nil, nil, 0, false, err
		}
	}
	changes := relevantChanges(selected)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Address < changes[j].Address })

	outputChanges := make([]report.OutputChange, 0, len(plan.OutputChanges))
	for name, outputChange := range plan.OutputChanges {
		if isNoOp(outputChange.Actions) {
			continue
		}
		outputChanges = append(outputChanges, report.OutputChange{Name: name, Actions: append([]string(nil), outputChange.Actions...)})
	}
	sort.Slice(outputChanges, func(i, j int) bool { return outputChanges[i].Name < outputChanges[j].Name })

	if total, exact := countPriorState(plan.PriorState); exact {
		return changes, outputChanges, total, true, nil
	}
	return changes, outputChanges, countManagedResources(resourceChanges), false, nil
}

func decodeResourceChanges(data json.RawMessage) ([]terraformResourceChange, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var changes []terraformResourceChange
	if err := json.Unmarshal(data, &changes); err != nil {
		return nil, fmt.Errorf("parse terraform resource changes: %w", err)
	}
	return changes, nil
}

func relevantChanges(source []terraformResourceChange) []report.ResourceChange {
	changes := make([]report.ResourceChange, 0, len(source))
	for _, resourceChange := range source {
		if resourceChange.Mode == "data" || isNoOp(resourceChange.Change.Actions) || isRead(resourceChange.Change.Actions) {
			continue
		}
		changes = append(changes, report.ResourceChange{
			Address:            resourceChange.Address,
			Type:               resourceChange.Type,
			Name:               resourceChange.Name,
			Actions:            append([]string(nil), resourceChange.Change.Actions...),
			ActionReason:       resourceChange.ActionReason,
			AttributeChanges:   attributeChangesFor(resourceChange.Change),
			Remediation:        report.RemediationForActions(resourceChange.Change.Actions),
			ReconciliationHint: report.ReconciliationHintForActions(resourceChange.Change.Actions),
			RiskLevel:          report.RiskLevelForActions(resourceChange.Change.Actions),
			Provider:           resourceChange.ProviderName,
			CloudProvider:      report.CloudProviderFor(resourceChange.ProviderName, resourceChange.Type),
		})
	}
	return changes
}

func countPriorState(state *terraformState) (int, bool) {
	if state == nil || state.Values == nil || state.Values.RootModule == nil {
		return 0, false
	}
	return countModule(*state.Values.RootModule), true
}

func countModule(module terraformModule) int {
	total := 0
	for _, resource := range module.Resources {
		if resource.Mode != "data" {
			total++
		}
	}
	for _, child := range module.ChildModules {
		total += countModule(child)
	}
	return total
}

func countManagedResources(resources []terraformResourceChange) int {
	total := 0
	for _, resource := range resources {
		if resource.Mode != "data" {
			total++
		}
	}
	return total
}

func isNoOp(actions []string) bool {
	return len(actions) == 0 || len(actions) == 1 && actions[0] == "no-op"
}

func isRead(actions []string) bool {
	return len(actions) == 1 && actions[0] == "read"
}
