package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

// DefaultPath is the default local TerraDrift configuration filename.
const DefaultPath = ".terradrift.json"

const maxConfigBytes = 1 << 20

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
	HistoryRetention     int                        `json:"history_retention"`
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
	AuditCommand         string                     `json:"audit_command"`
	AuditArgs            []string                   `json:"audit_args"`
	AllowedCommands      []string                   `json:"allowed_commands"`
	TrustedCommandDirs   []string                   `json:"trusted_command_dirs"`
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
	data, err := readConfig(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := Default()
	if err := decode(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if profile == "" {
		if err := cfg.Validate(); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	data, ok := cfg.Profiles[profile]
	if !ok {
		return Config{}, fmt.Errorf("config profile %q not found in %s", profile, path)
	}
	cfg = Default()
	if err := decode(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config profile %q in %s: %w", profile, path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func readConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigBytes {
		return nil, fmt.Errorf("config exceeds %d bytes", maxConfigBytes)
	}
	return data, nil
}

// Validate rejects invalid core settings before a scan starts.
func (cfg Config) Validate() error {
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil || timeout <= 0 {
		return fmt.Errorf("config timeout must be a positive duration")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Output)) {
	case "table", "json", "junit", "sarif", "prometheus":
	default:
		return fmt.Errorf("unsupported config output %q", cfg.Output)
	}
	if cfg.Notify != "" {
		switch strings.ToLower(strings.TrimSpace(cfg.Notify)) {
		case "slack", "teams", "webhook":
		default:
			return fmt.Errorf("unsupported config notify target %q", cfg.Notify)
		}
	}
	if cfg.FailureSeverity != "" {
		switch cfg.FailureSeverity {
		case "low", "medium", "high", "critical":
		default:
			return fmt.Errorf("unsupported config failure_severity %q", cfg.FailureSeverity)
		}
	}
	return nil
}

func decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
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
