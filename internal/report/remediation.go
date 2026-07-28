package report

import (
	"fmt"
	"net/url"
)

// RemediationForActions returns human-reviewed remediation guidance for Terraform drift actions.
func RemediationForActions(actions []string) string {
	if hasAction(actions, "delete") && hasAction(actions, "create") {
		return "Review replacement drift. Confirm whether to update Terraform configuration, import/sync state, or recreate only after approval."
	}
	if hasAction(actions, "delete") {
		return "Review deletion drift. Confirm whether to restore the resource, remove it from Terraform state/configuration, or accept the deletion after approval."
	}
	if hasAction(actions, "create") {
		return "Review unmanaged or missing resource drift. Confirm whether to import the resource, add Terraform configuration, or remove the unexpected resource after approval."
	}
	if hasAction(actions, "update") {
		return "Review update drift. Confirm whether to codify the changed settings in Terraform or revert the infrastructure change after approval."
	}
	if hasAction(actions, "read") {
		return "Review refreshed resource values and verify whether provider-side changes require Terraform configuration or state updates."
	}
	return "Review the drifted resource with the owning team before changing infrastructure or Terraform state."
}

// ApplyRunbooks attaches validated runbook URLs by resource type or type/action.
func ApplyRunbooks(scanReport *DriftReport, runbooks map[string]string) error {
	for i := range scanReport.ResourceChanges {
		change := &scanReport.ResourceChanges[i]
		runbookURL := runbooks[change.Type]
		for _, action := range change.Actions {
			if actionURL, ok := runbooks[change.Type+"/"+action]; ok {
				runbookURL = actionURL
				break
			}
		}
		if runbookURL == "" {
			continue
		}
		parsed, err := url.Parse(runbookURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("invalid runbook URL for %s: must be an HTTPS URL without user info", change.Type)
		}
		change.RunbookURL = parsed.String()
	}
	return nil
}

func hasAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}
