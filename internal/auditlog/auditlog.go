// Package auditlog writes secret-safe scan lifecycle records.
package auditlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/niravraychura/terradrift/internal/redact"
)

// Event contains the allowlisted metadata recorded for a scan lifecycle event.
type Event struct {
	Event            string   `json:"event"`
	ScanID           string   `json:"scan_id,omitempty"`
	Status           string   `json:"status,omitempty"`
	PlanMode         string   `json:"plan_mode,omitempty"`
	Workspace        string   `json:"workspace,omitempty"`
	Config           string   `json:"config,omitempty"`
	Profile          string   `json:"profile,omitempty"`
	TerraformVersion string   `json:"terraform_version,omitempty"`
	Commands         []string `json:"commands,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// Append writes one JSON Lines event with restrictive permissions.
func Append(path string, event Event) error {
	if path == "" {
		return nil
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("audit log must not be a symlink: %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect audit log: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create audit log directory: %w", err)
	}
	event.Error = redact.String(event.Error)
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure audit log: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}
