// Package report defines TerraDrift drift scan domain models.
package report

import "time"

// ScanStatus describes the lifecycle or result state of a drift scan.
type ScanStatus string

const (
	ScanStatusRunning       ScanStatus = "running"
	ScanStatusNoDrift       ScanStatus = "no_drift"
	ScanStatusDriftDetected ScanStatus = "drift_detected"
	ScanStatusFailed        ScanStatus = "failed"
)

// ResourceChange describes a Terraform resource with detected drift.
type ResourceChange struct {
	Address            string   `json:"address"`
	Type               string   `json:"type"`
	Name               string   `json:"name"`
	Actions            []string `json:"actions"`
	Remediation        string   `json:"remediation,omitempty"`
	ReconciliationHint string   `json:"reconciliation_hint,omitempty"`
	RunbookURL         string   `json:"runbook_url,omitempty"`
	CostImpact         string   `json:"cost_impact,omitempty"`
	Ignored            bool     `json:"ignored,omitempty"`
	IgnoreOwner        string   `json:"ignore_owner,omitempty"`
	IgnoreReason       string   `json:"ignore_reason,omitempty"`
	IgnoreExpiresAt    string   `json:"ignore_expires_at,omitempty"`
}

// IgnoreRule records a temporary, auditable exception for one resource address.
type IgnoreRule struct {
	Address   string `json:"address"`
	Owner     string `json:"owner"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at"`
}

// DriftReport captures the domain result of a Terraform drift scan.
type DriftReport struct {
	Status                ScanStatus       `json:"status"`
	Directory             string           `json:"directory"`
	TotalResourcesChecked int              `json:"total_resources_checked"`
	TotalChangedResources int              `json:"total_changed_resources"`
	ResourceChanges       []ResourceChange `json:"resource_changes"`
	StartedAt             time.Time        `json:"started_at"`
	CompletedAt           time.Time        `json:"completed_at"`
	ErrorMessage          string           `json:"error_message,omitempty"`
}
