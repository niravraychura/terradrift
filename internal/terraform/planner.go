package terraform

import (
	"os"
	"path/filepath"
	"strings"
)

// TerragruntConfigName is the stacked-root marker Terragrunt uses.
const TerragruntConfigName = "terragrunt.hcl"

const defaultTerragruntBin = "terragrunt"

// IsTerragruntRoot reports whether directory contains a Terragrunt config file.
func IsTerragruntRoot(directory string) bool {
	if strings.TrimSpace(directory) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(directory, TerragruntConfigName))
	return err == nil && !info.IsDir()
}

// PlannerPath selects the CLI to invoke for a root. Terragrunt roots use
// terragruntBin (default terragrunt), unless terraformBin is already a Terragrunt
// wrapper. Terraform/OpenTofu remains the planner behind that wrapper.
func PlannerPath(directory, terraformBin, terragruntBin string) string {
	if !IsTerragruntRoot(directory) {
		return terraformBin
	}
	if strings.TrimSpace(terragruntBin) != "" {
		return terragruntBin
	}
	if isTerragruntBinary(terraformBin) {
		return terraformBin
	}
	return defaultTerragruntBin
}

func isTerragruntBinary(path string) bool {
	return plannerBinaryName(path) == defaultTerragruntBin
}

func plannerBinaryName(path string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(strings.TrimSpace(path))), ".exe")
}
