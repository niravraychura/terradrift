package auditlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendRedactsSecretsAndUsesRestrictedPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	secret := "secret-token"
	if err := Append(path, Event{Event: "scan_failed", Error: "token=" + secret}); err != nil {
		t.Fatalf("append audit event: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 audit log, info=%v err=%v", info, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if strings.Contains(string(data), secret) || !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("expected redacted audit event, got %q", data)
	}
}
