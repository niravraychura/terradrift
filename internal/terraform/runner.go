// Package terraform defines abstractions for invoking the Terraform CLI.
package terraform

import "context"

// Runner executes Terraform commands needed by TerraDrift.
type Runner interface {
	// Init initializes the Terraform working directory.
	Init(ctx context.Context, directory string) error
	// PlanRefreshOnly writes a refresh-only plan and returns Terraform's detailed exit code.
	PlanRefreshOnly(ctx context.Context, directory string, outputPath string) (int, error)
	// ShowJSON returns the JSON rendering of a plan file.
	ShowJSON(ctx context.Context, directory string, planPath string) ([]byte, error)
}
