package redact

import (
	"strings"
	"testing"
)

func TestStringRedactsSensitiveAssignments(t *testing.T) {
	input := "token=abc123 password:super-secret api_key=key123 authorization=BearerValue client_secret=client-secret harmless=value"
	got := String(input)

	for _, leaked := range []string{"abc123", "super-secret", "key123", "BearerValue", "client-secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}
	if !strings.Contains(got, "harmless=value") {
		t.Fatalf("expected non-sensitive context to remain, got %q", got)
	}
}

func TestStringRedactsSlackWebhookURL(t *testing.T) {
	input := "posting to https://hooks.slack.com/services/T000/B000/secret-value now"
	got := String(input)

	if strings.Contains(got, "secret-value") || strings.Contains(got, "/services/") {
		t.Fatalf("expected Slack webhook path to be redacted, got %q", got)
	}
	if !strings.Contains(got, "hooks.slack.com") {
		t.Fatalf("expected host context to remain, got %q", got)
	}
}

func TestStringRedactsSensitiveURLQueryValues(t *testing.T) {
	input := "fetch https://example.com/callback?token=abc123&name=prod"
	got := String(input)

	if strings.Contains(got, "abc123") {
		t.Fatalf("expected token value to be redacted, got %q", got)
	}
	if !strings.Contains(got, "name=prod") {
		t.Fatalf("expected non-sensitive query value to remain, got %q", got)
	}
}
