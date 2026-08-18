package notify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/validation"
)

func TestGitHubPRNotifierPostsSummary(t *testing.T) {
	notifier := GitHubPRNotifier{
		Repository: "owner/repo",
		Number:     12,
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/repos/owner/repo/issues/12/comments" || request.Header.Get("Authorization") != "Bearer secret-token" {
				t.Fatalf("unexpected GitHub request: %s %#v", request.URL, request.Header)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), "Changed resources: 2") {
				t.Fatalf("unexpected summary: %q", body)
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), report.DriftReport{TotalChangedResources: 2}); err != nil {
		t.Fatalf("post pull request summary: %v", err)
	}
}

func TestGitHubNotifierReturnsTypedValidationErrors(t *testing.T) {
	err := GitHubPRNotifier{Repository: "invalid", Number: 1, Token: "token"}.Notify(context.Background(), report.DriftReport{})
	var validationErr *validation.Error
	if !errors.As(err, &validationErr) || validationErr.Field != "GitHub repository" {
		t.Fatalf("expected typed repository validation error, got %v", err)
	}
}

func TestGitHubIssueNotifierCreatesIssue(t *testing.T) {
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/repos/owner/repo/issues" || request.Header.Get("Authorization") != "Bearer secret-token" {
				t.Fatalf("unexpected GitHub request: %s %#v", request.URL, request.Header)
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), report.DriftReport{TotalChangedResources: 2}); err != nil {
		t.Fatalf("create drift issue: %v", err)
	}
}

func TestGitHubHTTPClientUsesSecureTimeout(t *testing.T) {
	client, err := githubHTTPClient(nil)
	if err != nil {
		t.Fatalf("githubHTTPClient: %v", err)
	}
	httpClient, ok := client.(*http.Client)
	if !ok {
		t.Fatalf("expected *http.Client, got %T", client)
	}
	if httpClient.Timeout != githubHTTPTimeout {
		t.Fatalf("timeout = %v, want %v", httpClient.Timeout, githubHTTPTimeout)
	}
	if httpClient == http.DefaultClient {
		t.Fatal("expected dedicated client, not http.DefaultClient")
	}
}

func TestGitHubNotifierRedactsTransportErrors(t *testing.T) {
	notifier := GitHubPRNotifier{
		Repository: "owner/repo",
		Number:     1,
		Token:      "secret-token",
		APIURL:     "https://api.github.com",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, errors.New("dial token=leaked-secret failed")
		}),
	}
	err := notifier.Notify(context.Background(), report.DriftReport{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "leaked-secret") {
		t.Fatalf("expected redacted transport error, got %v", err)
	}
}
