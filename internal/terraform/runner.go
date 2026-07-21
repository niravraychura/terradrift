// Package terraform defines abstractions for invoking the Terraform CLI.
package terraform

import "context"

// Runner executes Terraform commands needed by TerraDrift.
type Runner interface {
	Init(ctx context.Context, directory string) error
	PlanRefreshOnly(ctx context.Context, directory string, outputPath string) (int, error)
	ShowJSON(ctx context.Context, directory string, planPath string) ([]byte, error)
}
