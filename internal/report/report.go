// Package report defines TerraDrift drift scan domain models.
package report

import "time"

// ScanStatus describes the lifecycle or result state of a drift scan.
type ScanStatus string

const (
	ScanStatusPending       ScanStatus = "pending"
	ScanStatusRunning       ScanStatus = "running"
	ScanStatusNoDrift       ScanStatus = "no_drift"
	ScanStatusDriftDetected ScanStatus = "drift_detected"
	ScanStatusFailed        ScanStatus = "failed"
)

// ResourceChange describes a Terraform resource with detected drift.
type ResourceChange struct {
	Address string   `json:"address"`
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

// DriftReport captures the domain result of a Terraform drift scan.
type DriftReport struct {
	Status                ScanStatus       `json:"status"`
	TotalResourcesChecked int              `json:"total_resources_checked"`
	TotalChangedResources int              `json:"total_changed_resources"`
	ResourceChanges       []ResourceChange `json:"resource_changes"`
	StartedAt             time.Time        `json:"started_at"`
	CompletedAt           time.Time        `json:"completed_at"`
	ErrorMessage          string           `json:"error_message,omitempty"`
}
