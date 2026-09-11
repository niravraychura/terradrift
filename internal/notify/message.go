package notify

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

const (
	maxNotifyChanges = 15
	maxNotifyAttrs   = 8
)

// RedactedNotificationMessage formats a scan summary without leaking raw local paths.
// Attribute paths are included; before/after values are included only when present
// on the report (delivery already strips them unless --attribute-values).
func RedactedNotificationMessage(scanReport report.DriftReport) string {
	message := fmt.Sprintf("Terraform scan completed\nScan ID: %s\nStatus: %s\nPlan mode: %s\nResources checked: %d\nChanged resources: %d",
		scanReport.ScanID,
		scanReport.Status,
		scanReport.PlanMode,
		scanReport.TotalResourcesChecked,
		scanReport.TotalChangedResources,
	)
	if scanReport.ConfigStatus != "" {
		message += "\nConfig status: " + string(scanReport.ConfigStatus)
	}
	if duration := notifyDuration(scanReport); duration != "" {
		message += "\nDuration: " + duration
	}
	if riskLine := notifyRiskCounts(scanReport); riskLine != "" {
		message += "\nBy risk: " + riskLine
	}

	changes := notifyActiveChanges(scanReport)
	if len(changes) == 0 {
		return message
	}
	if notifyHasAttributeValues(changes) {
		message += "\n" + notifyAttributeSides(scanReport.PlanMode)
	}
	message += "\nChanges:"
	limit := maxNotifyChanges
	if len(changes) < limit {
		limit = len(changes)
	}
	for _, change := range changes[:limit] {
		message += "\n- " + notifyChangeLine(change)
		attrs := change.AttributeChanges
		attrLimit := maxNotifyAttrs
		if len(attrs) < attrLimit {
			attrLimit = len(attrs)
		}
		for _, attr := range attrs[:attrLimit] {
			if attr.Path == "" {
				continue
			}
			if attr.Before != "" || attr.After != "" {
				message += fmt.Sprintf("\n  %s: %s -> %s", attr.Path, notifyAttrValue(attr.Before), notifyAttrValue(attr.After))
				continue
			}
			message += "\n  " + attr.Path
		}
		if extra := len(attrs) - attrLimit; extra > 0 {
			message += fmt.Sprintf("\n  … and %d more attributes", extra)
		}
	}
	if extra := len(changes) - limit; extra > 0 {
		message += fmt.Sprintf("\n- … and %d more", extra)
	}
	return message
}

func notifyActiveChanges(scanReport report.DriftReport) []report.ResourceChange {
	changes := make([]report.ResourceChange, 0, len(scanReport.ResourceChanges))
	for _, change := range scanReport.ResourceChanges {
		if change.Ignored {
			continue
		}
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool {
		ri, rj := notifyRiskRank(changes[i].RiskLevel), notifyRiskRank(changes[j].RiskLevel)
		if ri != rj {
			return ri > rj
		}
		return changes[i].Address < changes[j].Address
	})
	return changes
}

func notifyChangeLine(change report.ResourceChange) string {
	risk := strings.ToUpper(strings.TrimSpace(change.RiskLevel))
	if risk == "" {
		risk = "UNKNOWN"
	}
	actions := strings.Join(change.Actions, ",")
	if actions == "" {
		actions = "update"
	}
	parts := []string{risk, actions}
	if kind := strings.TrimSpace(change.ChangeKind); kind != "" {
		parts = append(parts, kind)
	}
	if typeName := strings.TrimSpace(change.Type); typeName != "" {
		parts = append(parts, typeName)
	}
	if address := strings.TrimSpace(change.Address); address != "" {
		parts = append(parts, address)
	}
	return strings.Join(parts, "  ")
}

func notifyAttrValue(value string) string {
	if value == "" {
		return "(absent)"
	}
	return value
}

func notifyHasAttributeValues(changes []report.ResourceChange) bool {
	for _, change := range changes {
		for _, attr := range change.AttributeChanges {
			if attr.Before != "" || attr.After != "" {
				return true
			}
		}
	}
	return false
}

func notifyAttributeSides(planMode string) string {
	return "Attribute sides: " + report.AttributeSides(planMode)
}

func notifyDuration(scanReport report.DriftReport) string {
	if scanReport.StartedAt.IsZero() || scanReport.CompletedAt.IsZero() {
		return ""
	}
	elapsed := scanReport.CompletedAt.Sub(scanReport.StartedAt)
	if elapsed < time.Second {
		return elapsed.Round(time.Millisecond).String()
	}
	return elapsed.Round(time.Second).String()
}

func notifyRiskCounts(scanReport report.DriftReport) string {
	counts := map[string]int{}
	for _, change := range scanReport.ResourceChanges {
		if change.Ignored {
			continue
		}
		risk := strings.ToLower(strings.TrimSpace(change.RiskLevel))
		if risk == "" {
			risk = "unknown"
		}
		counts[risk]++
	}
	if len(counts) == 0 {
		return ""
	}
	order := []string{"critical", "high", "medium", "low", "unknown"}
	parts := make([]string, 0, len(order))
	for _, risk := range order {
		if n := counts[risk]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", risk, n))
		}
	}
	return strings.Join(parts, ", ")
}

func notifyRiskRank(level string) int {
	return map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}[strings.ToLower(strings.TrimSpace(level))]
}
