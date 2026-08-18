package redact

import (
	"strings"
	"testing"
)

func TestStringRedactsSensitiveAssignments(t *testing.T) {
	input := "token=abc123 password:super-secret api_key=key123 authorization=BearerValue client_secret=client-secret \"token\":\"json-secret\" 'password' = 'hcl-secret' harmless=value\nAuthorization: Bearer bearer-secret\nAuthorization: Basic basic-secret"
	got := String(input)

	for _, leaked := range []string{"abc123", "super-secret", "key123", "BearerValue", "bearer-secret", "basic-secret", "client-secret", "json-secret", "hcl-secret"} {
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

func TestStringRedactsTeamsWebhookURL(t *testing.T) {
	input := "posting to https://example.webhook.office.com/webhookb2/secret-value now"
	got := String(input)

	if strings.Contains(got, "secret-value") || strings.Contains(got, "webhookb2") {
		t.Fatalf("expected Teams webhook path to be redacted, got %q", got)
	}
	if !strings.Contains(got, "webhook.office.com") {
		t.Fatalf("expected host context to remain, got %q", got)
	}
}

func TestStringRedactsSensitiveURLQueryValues(t *testing.T) {
	input := "fetch https://example.com/callback?token=abc123&X-Amz-Signature=aws-signature&api_key=secret-key&name=prod&key_id=abc&key_name=production"
	got := String(input)

	if strings.Contains(got, "abc123") || strings.Contains(got, "aws-signature") || strings.Contains(got, "secret-key") {
		t.Fatalf("expected token value to be redacted, got %q", got)
	}
	if !strings.Contains(got, "name=prod") {
		t.Fatalf("expected non-sensitive query value to remain, got %q", got)
	}
	if !strings.Contains(got, "key_id=abc") || !strings.Contains(got, "key_name=production") {
		t.Fatalf("expected benign key_* parameters to remain, got %q", got)
	}
}

func TestIsSensitiveKeyExactMatches(t *testing.T) {
	for _, key := range []string{"api_key", "access_key", "AWS_ACCESS_KEY_ID", "private_key"} {
		if !isSensitiveKey(key) {
			t.Fatalf("expected %q to be sensitive", key)
		}
	}
	for _, key := range []string{"key_id", "key_name", "pagerduty_key", "keyboard"} {
		if isSensitiveKey(key) {
			t.Fatalf("expected %q not to be sensitive", key)
		}
	}
}
