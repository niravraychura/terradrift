package parser

import (
	"encoding/json"
	"fmt"

	"github.com/niravraychura/terradrift/internal/report"
)

type terraformPlan struct {
	ResourceChanges []terraformResourceChange `json:"resource_changes"`
}

type terraformResourceChange struct {
	Address string          `json:"address"`
	Type    string          `json:"type"`
	Name    string          `json:"name"`
	Change  terraformChange `json:"change"`
}

type terraformChange struct {
	Actions []string `json:"actions"`
}

// ParsePlan converts the subset of Terraform plan JSON needed for drift reports.
func ParsePlan(data []byte) ([]report.ResourceChange, int, error) {
	var plan terraformPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, 0, fmt.Errorf("parse terraform plan JSON: %w", err)
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
			Remediation:        report.RemediationForActions(resourceChange.Change.Actions),
			ReconciliationHint: report.ReconciliationHintForActions(resourceChange.Change.Actions),
		})
	}

	return changes, len(plan.ResourceChanges), nil
}

func isNoOp(actions []string) bool {
	return len(actions) == 0 || len(actions) == 1 && actions[0] == "no-op"
}
