package parser

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/niravraychura/terradrift/internal/report"
)

type terraformPlan struct {
	ResourceChanges []terraformResourceChange  `json:"resource_changes"`
	OutputChanges   map[string]terraformChange `json:"output_changes"`
}

type terraformResourceChange struct {
	Address      string          `json:"address"`
	Type         string          `json:"type"`
	Name         string          `json:"name"`
	ProviderName string          `json:"provider_name"`
	ActionReason string          `json:"action_reason"`
	Change       terraformChange `json:"change"`
}

type terraformChange struct {
	// Keep the decoded shape intentionally narrow: Terraform values and sensitive marks never enter reports.
	Actions []string `json:"actions"`
}

// ParsePlan converts the subset of Terraform plan JSON needed for drift reports.
func ParsePlan(data []byte) ([]report.ResourceChange, []report.OutputChange, int, error) {
	var plan terraformPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, nil, 0, fmt.Errorf("parse terraform plan JSON: %w", err)
	}

	changes := make([]report.ResourceChange, 0, len(plan.ResourceChanges))
	for _, resourceChange := range plan.ResourceChanges {
		if isNoOp(resourceChange.Change.Actions) {
			continue
		}
		changes = append(changes, report.ResourceChange{
			Address:            resourceChange.Address,
			Type:               resourceChange.Type,
			Name:               resourceChange.Name,
			Actions:            append([]string(nil), resourceChange.Change.Actions...),
			ActionReason:       resourceChange.ActionReason,
			Remediation:        report.RemediationForActions(resourceChange.Change.Actions),
			ReconciliationHint: report.ReconciliationHintForActions(resourceChange.Change.Actions),
			RiskLevel:          report.RiskLevelForActions(resourceChange.Change.Actions),
			Provider:           resourceChange.ProviderName,
			CloudProvider:      report.CloudProviderFor(resourceChange.ProviderName, resourceChange.Type),
		})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Address < changes[j].Address })

	outputChanges := make([]report.OutputChange, 0, len(plan.OutputChanges))
	for name, outputChange := range plan.OutputChanges {
		if isNoOp(outputChange.Actions) {
			continue
		}
		outputChanges = append(outputChanges, report.OutputChange{Name: name, Actions: append([]string(nil), outputChange.Actions...)})
	}
	sort.Slice(outputChanges, func(i, j int) bool { return outputChanges[i].Name < outputChanges[j].Name })

	return changes, outputChanges, len(plan.ResourceChanges), nil
}

func isNoOp(actions []string) bool {
	return len(actions) == 0 || len(actions) == 1 && actions[0] == "no-op"
}
