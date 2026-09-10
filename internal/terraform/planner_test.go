package terraform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTerragruntRoot(t *testing.T) {
	root := t.TempDir()
	if IsTerragruntRoot(root) || IsTerragruntRoot("") {
		t.Fatal("expected empty directory not to be a Terragrunt root")
	}
	if err := os.WriteFile(filepath.Join(root, TerragruntConfigName), []byte("terraform { source = \".\" }\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if !IsTerragruntRoot(root) {
		t.Fatal("expected terragrunt.hcl to mark a Terragrunt root")
	}
}

func TestPlannerPath(t *testing.T) {
	tfRoot := t.TempDir()
	tgRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(tgRoot, TerragruntConfigName), []byte("# synthetic\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if got := PlannerPath(tfRoot, "tofu", "/opt/terragrunt"); got != "tofu" {
		t.Fatalf("terraform root: got %q", got)
	}
	if got := PlannerPath(tgRoot, "tofu", ""); got != "terragrunt" {
		t.Fatalf("terragrunt default: got %q", got)
	}
	if got := PlannerPath(tgRoot, "tofu", "/opt/terragrunt"); got != "/opt/terragrunt" {
		t.Fatalf("terragrunt override: got %q", got)
	}
	if got := PlannerPath(tgRoot, "/usr/bin/terragrunt", ""); got != "/usr/bin/terragrunt" {
		t.Fatalf("terraform-bin passthrough: got %q", got)
	}
}
