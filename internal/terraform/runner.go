// Package terraform defines abstractions for invoking the Terraform CLI.
package terraform

import (
	"context"
	"fmt"
	"strings"
)

// PlanMode controls which Terraform reconciliation view is reported.
type PlanMode string

const (
	PlanModeRefreshOnly PlanMode = "refresh-only"
	PlanModeNormal      PlanMode = "normal"
)

// ParsePlanMode validates a user-supplied plan mode. An empty mode defaults to refresh-only.
func ParsePlanMode(value string) (PlanMode, error) {
	mode := PlanMode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		return PlanModeRefreshOnly, nil
	}
	switch mode {
	case PlanModeRefreshOnly, PlanModeNormal:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported plan mode %q; supported values: refresh-only, normal", value)
	}
}

// Runner executes Terraform commands needed by TerraDrift.
type Runner interface {
	// Init initializes the Terraform working directory.
	Init(ctx context.Context, directory string) error
	// Plan writes a plan for mode and returns Terraform's detailed exit code.
	Plan(ctx context.Context, directory string, outputPath string, mode PlanMode) (int, error)
	// ShowJSON returns the JSON rendering of a plan file.
	ShowJSON(ctx context.Context, directory string, planPath string) ([]byte, error)
}
