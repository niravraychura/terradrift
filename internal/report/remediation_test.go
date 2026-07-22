package report

import (
	"strings"
	"testing"
)

func TestRemediationForActions(t *testing.T) {
	cases := map[string][]string{
		"update drift":      {"update"},
		"replacement drift": {"delete", "create"},
		"deletion drift":    {"delete"},
		"missing resource":  {"create"},
	}
	for want, actions := range cases {
		t.Run(want, func(t *testing.T) {
			got := strings.ToLower(RemediationForActions(actions))
			if !strings.Contains(got, "review") || !strings.Contains(got, "approval") {
				t.Fatalf("expected reviewed remediation guidance, got %q", got)
			}
		})
	}
}
