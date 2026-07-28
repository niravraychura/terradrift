// Package report defines TerraDrift drift scan domain models.
package report

import "time"

// ScanStatus describes the lifecycle or result state of a drift scan.
type ScanStatus string

const (
	ScanStatusRunning         ScanStatus = "running"
	ScanStatusNoDrift         ScanStatus = "no_drift"
	ScanStatusDriftDetected   ScanStatus = "drift_detected"
	ScanStatusNoChanges       ScanStatus = "no_changes"
	ScanStatusChangesDetected ScanStatus = "changes_detected"
	ScanStatusFailed          ScanStatus = "failed"
)

// ResourceChange describes a Terraform resource with a relevant plan change.
type ResourceChange struct {
	Address            string       `json:"address"`
	Type               string       `json:"type"`
	Name               string       `json:"name"`
	Actions            []string     `json:"actions"`
	ActionReason       string       `json:"action_reason,omitempty"`
	Remediation        string       `json:"remediation,omitempty"`
	ReconciliationHint string       `json:"reconciliation_hint,omitempty"`
	RunbookURL         string       `json:"runbook_url,omitempty"`
	CostImpact         string       `json:"cost_impact,omitempty"`
	RiskLevel          string       `json:"risk_level,omitempty"`
	Owner              string       `json:"owner,omitempty"`
	Provider           string       `json:"provider,omitempty"`
	CloudProvider      string       `json:"cloud_provider,omitempty"`
	AuditEvents        []AuditEvent `json:"audit_events,omitempty"`
	Ignored            bool         `json:"ignored,omitempty"`
	IgnoreOwner        string       `json:"ignore_owner,omitempty"`
	IgnoreReason       string       `json:"ignore_reason,omitempty"`
	IgnoreExpiresAt    string       `json:"ignore_expires_at,omitempty"`
}

// OutputChange describes a Terraform output change without retaining its value.
type OutputChange struct {
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

// IgnoreRule records a temporary, auditable exception for one resource address.
type IgnoreRule struct {
	Address   string `json:"address"`
	Owner     string `json:"owner"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at"`
}

// ModuleInventory identifies one initialized Terraform module without local paths.
type ModuleInventory struct {
	Key     string `json:"key"`
	Source  string `json:"source"`
	Version string `json:"version,omitempty"`
}

// AuditEvent identifies an external cloud audit event for a resource.
type AuditEvent struct {
	Provider   string `json:"provider"`
	Actor      string `json:"actor"`
	OccurredAt string `json:"occurred_at"`
	Summary    string `json:"summary"`
}

// DriftReport captures the domain result of a Terraform scan.
type DriftReport struct {
	ScanID                string            `json:"scan_id"`
	RootID                string            `json:"root_id,omitempty"`
	Status                ScanStatus        `json:"status"`
	Directory             string            `json:"directory"`
	PlanMode              string            `json:"plan_mode"`
	TotalResourcesChecked int               `json:"total_resources_checked"`
	ResourcesCheckedExact bool              `json:"resources_checked_exact"`
	TotalChangedResources int               `json:"total_changed_resources"`
	ResourceChanges       []ResourceChange  `json:"resource_changes"`
	OutputChanges         []OutputChange    `json:"output_changes,omitempty"`
	StartedAt             time.Time         `json:"started_at"`
	CompletedAt           time.Time         `json:"completed_at"`
	ErrorMessage          string            `json:"error_message,omitempty"`
	TerraformVersion      string            `json:"terraform_version,omitempty"`
	ProviderVersions      map[string]string `json:"provider_versions,omitempty"`
	Modules               []ModuleInventory `json:"modules,omitempty"`
	Approval              *Approval         `json:"approval,omitempty"`
}

// HasChanges reports whether a completed scan contains active findings.
func HasChanges(status ScanStatus) bool {
	return status == ScanStatusDriftDetected || status == ScanStatusChangesDetected
}
