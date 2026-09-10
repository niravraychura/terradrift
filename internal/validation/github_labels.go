package validation

import (
	"fmt"
	"regexp"
)

// MaxGitHubIssueLabels caps optional labels on persistent-drift issues.
const MaxGitHubIssueLabels = 8

var githubIssueLabelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,49}$`)

// GitHubIssueLabels rejects an unbounded or malformed label set without echoing values.
func GitHubIssueLabels(labels []string) error {
	if len(labels) > MaxGitHubIssueLabels {
		return New("github_issue_labels", fmt.Errorf("at most %d labels", MaxGitHubIssueLabels))
	}
	for _, label := range labels {
		if !githubIssueLabelName.MatchString(label) {
			return New("github_issue_labels", fmt.Errorf("must be 1-50 characters matching [A-Za-z0-9._-]"))
		}
	}
	return nil
}
