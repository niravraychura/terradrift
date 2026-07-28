package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDefaultAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := WriteDefault(path); err != nil {
		t.Fatalf("write default config: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected config permissions 0600, got %o", got)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Directory != "." || cfg.Output != "table" || cfg.Timeout != "5m" || cfg.RedactPaths || cfg.TerraformExec || cfg.TerraformBin != "" || cfg.WorkspaceRoot != "" || cfg.Notify != "" || cfg.SlackWebhookURL != "" || cfg.TeamsWebhookURL != "" || cfg.WebhookURL != "" || cfg.DashboardHTML != "" || cfg.HistoryDir != "" || cfg.PolicyCommand != "" || cfg.PolicyArgs != nil || cfg.CostCommand != "" || cfg.CostArgs != nil || cfg.RemediationRunbooks != nil || cfg.Profiles != nil {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`{"profiles":{"production":{"directory":"./prod","output":"json","terraform_exec":true}}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	cfg, err := LoadProfile(path, "production")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if cfg.Directory != "./prod" || cfg.Output != "json" || !cfg.TerraformExec {
		t.Fatalf("unexpected profile: %#v", cfg)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(`{"directory":`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(`{"direcotry":"."}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown config field to fail")
	}
}

func TestLoadRejectsInvalidUnselectedProfileAndTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(`{"profiles":{"unused":{"typo":true}}}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid unselected profile to fail")
	}
	if err := os.WriteFile(path, []byte(`{} {}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected trailing JSON to fail")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(`{"output":"xml"}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid config value to fail")
	}
}

func TestLoadRejectsUnsafeConfiguredURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(`{"webhook_url":"http://example.com"}`), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unsafe URL to fail")
	}
}

func TestLoadBaselineRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`{"baseline_rules":[{"address":"aws_instance.web","owner":"platform","reason":"accepted","expires_at":"2030-01-01T00:00:00Z"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	cfg, err := Load(path)
	if err != nil || len(cfg.BaselineRules) != 1 || cfg.BaselineRules[0].Address != "aws_instance.web" {
		t.Fatalf("unexpected baseline config: %#v, %v", cfg, err)
	}
}

func TestLoadRejectsOversizedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, make([]byte, maxConfigBytes+1), 0o600); err != nil {
		t.Fatalf("write oversized config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected oversized config to fail")
	}
}
