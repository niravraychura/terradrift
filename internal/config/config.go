package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/niravraychura/terradrift/internal/report"
)

const DefaultPath = ".terradrift.json"

// Config stores repeatable TerraDrift CLI settings for local and CI usage.
type Config struct {
	Directory            string                     `json:"directory"`
	Output               string                     `json:"output"`
	Timeout              string                     `json:"timeout"`
	RedactPaths          bool                       `json:"redact_paths"`
	TerraformExec        bool                       `json:"terraform_exec"`
	TerraformBin         string                     `json:"terraform_bin"`
	WorkspaceRoot        string                     `json:"workspace_root"`
	Notify               string                     `json:"notify"`
	SlackWebhookURL      string                     `json:"slack_webhook_url"`
	TeamsWebhookURL      string                     `json:"teams_webhook_url"`
	WebhookURL           string                     `json:"webhook_url"`
	DashboardHTML        string                     `json:"dashboard_html"`
	HistoryDir           string                     `json:"history_dir"`
	PolicyCommand        string                     `json:"policy_command"`
	PolicyArgs           []string                   `json:"policy_args"`
	CostCommand          string                     `json:"cost_command"`
	CostArgs             []string                   `json:"cost_args"`
	RemediationRunbooks  map[string]string          `json:"remediation_runbooks"`
	IgnoreRules          []report.IgnoreRule        `json:"ignore_rules"`
	FailureSeverity      string                     `json:"failure_severity"`
	ResourceOwners       map[string]string          `json:"resource_owners"`
	OwnerWebhooks        map[string]string          `json:"owner_webhooks"`
	NotificationThrottle bool                       `json:"notification_throttle"`
	GitHubRepository     string                     `json:"github_repository"`
	GitHubPR             int                        `json:"github_pr"`
	GitHubIssueAfter     int                        `json:"github_issue_after"`
	ArtifactURL          string                     `json:"artifact_url"`
	Profiles             map[string]json.RawMessage `json:"profiles,omitempty"`
}

// Default returns a safe bootstrap configuration.
func Default() Config {
	return Config{Directory: ".", Output: "table", Timeout: "5m", RedactPaths: false}
}

// Load reads a TerraDrift JSON configuration file.
func Load(path string) (Config, error) {
	return LoadProfile(path, "")
}

// LoadProfile reads a config file and optionally selects a standalone named profile.
func LoadProfile(path string, profile string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if profile == "" {
		return cfg, nil
	}
	data, ok := cfg.Profiles[profile]
	if !ok {
		return Config{}, fmt.Errorf("config profile %q not found in %s", profile, path)
	}
	cfg = Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config profile %q in %s: %w", profile, path, err)
	}
	return cfg, nil
}

// WriteDefault writes the default configuration to path without overwriting existing files.
func WriteDefault(path string) error {
	if path == "" {
		path = DefaultPath
	}
	data, err := json.MarshalIndent(Default(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode default config: %w", err)
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("config already exists: %s", path)
		}
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %s: %w", path, err)
	}
	return nil
}
