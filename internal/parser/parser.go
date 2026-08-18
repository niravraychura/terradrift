package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/terraform"
)

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
//
// Parsing is token-streamed: unused top-level fields (configuration, planned_values,
// etc.) and prior_state resource value blobs are skipped without materializing them.
func ParsePlan(data []byte, mode terraform.PlanMode) ([]report.ResourceChange, []report.OutputChange, int, bool, error) {
	return ParsePlanReader(bytes.NewReader(data), mode)
}

// ParsePlanReader is the streaming entry point for terraform show -json output.
func ParsePlanReader(reader io.Reader, mode terraform.PlanMode) ([]report.ResourceChange, []report.OutputChange, int, bool, error) {
	mode, err := terraform.ParsePlanMode(string(mode))
	if err != nil {
		return nil, nil, 0, false, err
	}
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil {
		return nil, nil, 0, false, fmt.Errorf("parse terraform plan JSON: %w", err)
	}
	if token != json.Delim('{') {
		return nil, nil, 0, false, fmt.Errorf("parse terraform plan JSON: expected object")
	}

	var resourceChanges []terraformResourceChange
	var resourceDrift []terraformResourceChange
	var haveResourceDrift bool
	var outputChanges map[string]terraformChange
	priorTotal := 0
	priorExact := false

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, nil, 0, false, fmt.Errorf("parse terraform plan JSON: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, nil, 0, false, fmt.Errorf("parse terraform plan JSON: expected object key")
		}
		switch key {
		case "resource_changes":
			resourceChanges, err = decodeResourceChangesArray(decoder)
		case "resource_drift":
			haveResourceDrift = true
			resourceDrift, err = decodeResourceChangesArray(decoder)
		case "output_changes":
			err = decoder.Decode(&outputChanges)
			if err != nil {
				err = fmt.Errorf("parse terraform output changes: %w", err)
			}
		case "prior_state":
			priorTotal, priorExact, err = countPriorStateFromDecoder(decoder)
		default:
			err = skipValue(decoder)
		}
		if err != nil {
			return nil, nil, 0, false, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, nil, 0, false, fmt.Errorf("parse terraform plan JSON: %w", err)
	}

	selected := resourceChanges
	if mode == terraform.PlanModeRefreshOnly && haveResourceDrift {
		selected = resourceDrift
	}
	changes := relevantChanges(selected)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Address < changes[j].Address })

	outputs := make([]report.OutputChange, 0, len(outputChanges))
	for name, outputChange := range outputChanges {
		if isNoOp(outputChange.Actions) {
			continue
		}
		outputs = append(outputs, report.OutputChange{Name: name, Actions: append([]string(nil), outputChange.Actions...)})
	}
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].Name < outputs[j].Name })

	if priorExact {
		return changes, outputs, priorTotal, true, nil
	}
	return changes, outputs, countManagedResources(resourceChanges), false, nil
}

func decodeResourceChangesArray(decoder *json.Decoder) ([]terraformResourceChange, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("parse terraform resource changes: %w", err)
	}
	if token == nil {
		return nil, nil
	}
	if token != json.Delim('[') {
		return nil, fmt.Errorf("parse terraform resource changes: expected array")
	}
	changes := make([]terraformResourceChange, 0)
	for decoder.More() {
		var change terraformResourceChange
		if err := decoder.Decode(&change); err != nil {
			return nil, fmt.Errorf("parse terraform resource changes: %w", err)
		}
		changes = append(changes, change)
	}
	if _, err := decoder.Token(); err != nil {
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

func countPriorStateFromDecoder(decoder *json.Decoder) (int, bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, false, err
	}
	if token == nil {
		return 0, false, nil
	}
	if token != json.Delim('{') {
		if err := skipValueAfterToken(decoder, token); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}

	total := 0
	exact := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, false, err
		}
		key, _ := keyToken.(string)
		if key == "values" {
			total, exact, err = countPriorValuesFromDecoder(decoder)
			if err != nil {
				return 0, false, err
			}
			continue
		}
		if err := skipValue(decoder); err != nil {
			return 0, false, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return 0, false, err
	}
	return total, exact, nil
}

func countPriorValuesFromDecoder(decoder *json.Decoder) (int, bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, false, err
	}
	if token == nil {
		return 0, false, nil
	}
	if token != json.Delim('{') {
		if err := skipValueAfterToken(decoder, token); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}

	total := 0
	exact := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, false, err
		}
		key, _ := keyToken.(string)
		if key == "root_module" {
			count, err := countModuleFromDecoder(decoder)
			if err != nil {
				return 0, false, err
			}
			total = count
			exact = true
			continue
		}
		if err := skipValue(decoder); err != nil {
			return 0, false, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return 0, false, err
	}
	return total, exact, nil
}

func countModuleFromDecoder(decoder *json.Decoder) (int, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, err
	}
	if token == nil {
		return 0, nil
	}
	if token != json.Delim('{') {
		return 0, skipValueAfterToken(decoder, token)
	}

	total := 0
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, err
		}
		key, _ := keyToken.(string)
		switch key {
		case "resources":
			count, err := countResourcesArrayFromDecoder(decoder)
			if err != nil {
				return 0, err
			}
			total += count
		case "child_modules":
			count, err := countChildModulesArrayFromDecoder(decoder)
			if err != nil {
				return 0, err
			}
			total += count
		default:
			if err := skipValue(decoder); err != nil {
				return 0, err
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return 0, err
	}
	return total, nil
}

func countResourcesArrayFromDecoder(decoder *json.Decoder) (int, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, err
	}
	if token == nil {
		return 0, nil
	}
	if token != json.Delim('[') {
		return 0, skipValueAfterToken(decoder, token)
	}
	total := 0
	for decoder.More() {
		mode, err := resourceModeFromDecoder(decoder)
		if err != nil {
			return 0, err
		}
		if mode != "data" {
			total++
		}
	}
	if _, err := decoder.Token(); err != nil {
		return 0, err
	}
	return total, nil
}

func countChildModulesArrayFromDecoder(decoder *json.Decoder) (int, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, err
	}
	if token == nil {
		return 0, nil
	}
	if token != json.Delim('[') {
		return 0, skipValueAfterToken(decoder, token)
	}
	total := 0
	for decoder.More() {
		count, err := countModuleFromDecoder(decoder)
		if err != nil {
			return 0, err
		}
		total += count
	}
	if _, err := decoder.Token(); err != nil {
		return 0, err
	}
	return total, nil
}

func resourceModeFromDecoder(decoder *json.Decoder) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}
	if token != json.Delim('{') {
		if err := skipValueAfterToken(decoder, token); err != nil {
			return "", err
		}
		return "", nil
	}
	mode := ""
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return "", err
		}
		key, _ := keyToken.(string)
		if key == "mode" {
			modeToken, err := decoder.Token()
			if err != nil {
				return "", err
			}
			mode, _ = modeToken.(string)
			continue
		}
		if err := skipValue(decoder); err != nil {
			return "", err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return "", err
	}
	return mode, nil
}

// skipValue discards the next JSON value from decoder without retaining it.
func skipValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	return skipValueAfterToken(decoder, token)
}

func skipValueAfterToken(decoder *json.Decoder, token json.Token) error {
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		for decoder.More() {
			if _, err := decoder.Token(); err != nil {
				return err
			}
			if err := skipValue(decoder); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := skipValue(decoder); err != nil {
				return err
			}
		}
		_, err := decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
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
