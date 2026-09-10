package validation

import (
	"errors"
	"strings"
	"testing"
)

func TestNewErrorMessage(t *testing.T) {
	err := New("scan timeout", errors.New("must not be negative"))
	if err == nil || !strings.Contains(err.Error(), "invalid scan timeout") || !strings.Contains(err.Error(), "must not be negative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestErrorUnwrapAndAs(t *testing.T) {
	cause := errors.New("missing value")
	err := New("directory", cause)
	if !errors.Is(err, cause) {
		t.Fatal("expected errors.Is to match cause")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Field != "directory" {
		t.Fatalf("expected typed Error, got %#v", typed)
	}
	if typed.Unwrap() != cause {
		t.Fatalf("Unwrap() = %v, want %v", typed.Unwrap(), cause)
	}
}

func TestGitHubIssueLabels(t *testing.T) {
	if err := GitHubIssueLabels([]string{"terradrift", "drift"}); err != nil {
		t.Fatalf("expected valid labels, got %v", err)
	}
	if err := GitHubIssueLabels(make([]string, MaxGitHubIssueLabels+1)); err == nil {
		t.Fatal("expected too many labels to fail")
	}
	if err := GitHubIssueLabels([]string{"bad label"}); err == nil {
		t.Fatal("expected invalid label to fail")
	}
}
