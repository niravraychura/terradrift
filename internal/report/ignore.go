package report

import (
	"fmt"
	"strings"
	"time"
)

// ApplyIgnoreRules annotates active temporary exceptions and recalculates drift counts.
func ApplyIgnoreRules(scanReport *DriftReport, rules []IgnoreRule) error {
	active := make(map[string]IgnoreRule, len(rules))
	now := time.Now().UTC()
	for _, rule := range rules {
		rule.Address = strings.TrimSpace(rule.Address)
		rule.Owner = strings.TrimSpace(rule.Owner)
		rule.Reason = strings.TrimSpace(rule.Reason)
		if rule.Address == "" || rule.Owner == "" || rule.Reason == "" || rule.ExpiresAt == "" {
			return fmt.Errorf("ignore rules require address, owner, reason, and expires_at")
		}
		expiresAt, err := time.Parse(time.RFC3339, rule.ExpiresAt)
		if err != nil || !expiresAt.After(now) {
			return fmt.Errorf("ignore rule for %s must have a future RFC3339 expires_at", rule.Address)
		}
		active[rule.Address] = rule
	}

	changed := 0
	for i := range scanReport.ResourceChanges {
		change := &scanReport.ResourceChanges[i]
		rule, ok := active[change.Address]
		if !ok {
			changed++
			continue
		}
		change.Ignored = true
		change.IgnoreOwner = rule.Owner
		change.IgnoreReason = rule.Reason
		change.IgnoreExpiresAt = rule.ExpiresAt
	}
	scanReport.TotalChangedResources = changed
	if changed == 0 && scanReport.Status == ScanStatusDriftDetected {
		scanReport.Status = ScanStatusNoDrift
	}
	return nil
}

// ApplyOwners assigns exact-address owners before resource-type owners.
func ApplyOwners(scanReport *DriftReport, owners map[string]string) {
	for i := range scanReport.ResourceChanges {
		change := &scanReport.ResourceChanges[i]
		change.Owner = owners[change.Address]
		if change.Owner == "" {
			change.Owner = owners[change.Type]
		}
	}
}
