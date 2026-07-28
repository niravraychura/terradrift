package report

import (
	"fmt"
	"strings"
	"time"
)

// Approval records a human review of one active drift fingerprint.
type Approval struct {
	Fingerprint string `json:"fingerprint"`
	Owner       string `json:"owner"`
	Reason      string `json:"reason"`
	ExpiresAt   string `json:"expires_at"`
}

// NewApproval creates an expiring approval for a report's active drift.
func NewApproval(scanReport DriftReport, owner string, reason string, expiresAt string) (Approval, error) {
	approval := Approval{Fingerprint: DriftFingerprint(scanReport), Owner: strings.TrimSpace(owner), Reason: strings.TrimSpace(reason), ExpiresAt: expiresAt}
	if approval.Fingerprint == "" || approval.Owner == "" || approval.Reason == "" {
		return Approval{}, fmt.Errorf("approval requires active drift, owner, and reason")
	}
	if err := VerifyApproval(scanReport, approval); err != nil {
		return Approval{}, err
	}
	return approval, nil
}

// VerifyApproval confirms that an approval is current and matches the report.
func VerifyApproval(scanReport DriftReport, approval Approval) error {
	expiresAt, err := time.Parse(time.RFC3339, approval.ExpiresAt)
	if err != nil || !expiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("approval must have a future RFC3339 expires_at")
	}
	if approval.Fingerprint != DriftFingerprint(scanReport) {
		return fmt.Errorf("approval does not match active drift")
	}
	return nil
}
