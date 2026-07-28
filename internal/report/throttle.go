package report

import (
	"sort"
	"strings"
)

// ShouldNotify returns false only for unchanged active drift with no risk escalation.
func ShouldNotify(current DriftReport, previous DriftReport) bool {
	if current.Status != ScanStatusDriftDetected || previous.Status != ScanStatusDriftDetected {
		return true
	}
	currentFindings, currentRisk := activeFingerprint(current)
	previousFindings, previousRisk := activeFingerprint(previous)
	return currentFindings != previousFindings || severityRank(currentRisk) > severityRank(previousRisk)
}

func activeFingerprint(scanReport DriftReport) (string, string) {
	findings := make([]string, 0, len(scanReport.ResourceChanges))
	highestRisk := "low"
	for _, change := range scanReport.ResourceChanges {
		if change.Ignored {
			continue
		}
		risk := change.RiskLevel
		if risk == "" {
			risk = RiskLevelForActions(change.Actions)
		}
		if severityRank(risk) > severityRank(highestRisk) {
			highestRisk = risk
		}
		findings = append(findings, change.Address+":"+strings.Join(change.Actions, ",")+":"+risk)
	}
	sort.Strings(findings)
	return strings.Join(findings, "|"), highestRisk
}

func severityRank(level string) int {
	return map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}[level]
}
