package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
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
