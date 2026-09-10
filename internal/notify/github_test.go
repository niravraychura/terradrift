package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/validation"
)

func TestGitHubPRNotifierPostsSummary(t *testing.T) {
	calls := 0
	notifier := GitHubPRNotifier{
		Repository: "owner/repo",
		Number:     12,
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Header.Get("Authorization") != "Bearer secret-token" {
				t.Fatalf("unexpected GitHub auth: %#v", request.Header)
			}
			if calls == 1 {
				if request.Method != http.MethodGet || request.URL.Path != "/repos/owner/repo/issues/12/comments" {
					t.Fatalf("expected comment list, got %s %s", request.Method, request.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("[]"))}, nil
			}
			if request.Method != http.MethodPost || request.URL.Path != "/repos/owner/repo/issues/12/comments" {
				t.Fatalf("unexpected GitHub request: %s %s", request.Method, request.URL)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), "Changed resources: 2") || !strings.Contains(string(body), "terradrift-pr-comment") {
				t.Fatalf("unexpected summary: %q", body)
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), report.DriftReport{TotalChangedResources: 2}); err != nil {
		t.Fatalf("post pull request summary: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected list then create, got %d calls", calls)
	}
}

func TestGitHubPRNotifierPatchesExistingComment(t *testing.T) {
	calls := 0
	notifier := GitHubPRNotifier{
		Repository: "owner/repo",
		Number:     12,
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				if request.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", request.Method)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`[{"id":99,"body":"## TerraDrift Scan\nold"}]`))}, nil
			}
			if request.Method != http.MethodPatch || request.URL.Path != "/repos/owner/repo/issues/comments/99" {
				t.Fatalf("unexpected upsert: %s %s", request.Method, request.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), report.DriftReport{TotalChangedResources: 1}); err != nil {
		t.Fatalf("upsert pull request summary: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected list then patch, got %d calls", calls)
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
	calls := 0
	scanReport := report.DriftReport{
		RootID:                "abc123",
		TotalChangedResources: 2,
		Status:                report.ScanStatusDriftDetected,
		ResourceChanges:       []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"}},
	}
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Labels:     []string{"terradrift"},
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Header.Get("Authorization") != "Bearer secret-token" {
				t.Fatalf("unexpected GitHub auth: %#v", request.Header)
			}
			if calls == 1 {
				if request.Method != http.MethodGet || request.URL.Path != "/repos/owner/repo/issues" {
					t.Fatalf("expected issue list, got %s %s", request.Method, request.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("[]"))}, nil
			}
			if request.Method != http.MethodPost || request.URL.Path != "/repos/owner/repo/issues" {
				t.Fatalf("unexpected GitHub request: %s %s", request.Method, request.URL)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), "terradrift-issue root=abc123") || !strings.Contains(string(body), `"labels":["terradrift"]`) {
				t.Fatalf("unexpected issue payload: %q", body)
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), scanReport); err != nil {
		t.Fatalf("create drift issue: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected list then create, got %d calls", calls)
	}
}

func TestGitHubIssueNotifierUpsertsMatchingIssue(t *testing.T) {
	scanReport := report.DriftReport{
		RootID:          "abc123",
		Status:          report.ScanStatusDriftDetected,
		ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"}},
	}
	marker := githubIssueMarker("abc123", githubIssueFPHash(scanReport))
	calls := 0
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				payload := fmt.Sprintf(`[{"number":44,"body":%q}]`, marker+"\nold")
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(payload))}, nil
			}
			if request.Method != http.MethodPatch || request.URL.Path != "/repos/owner/repo/issues/44" {
				t.Fatalf("unexpected upsert: %s %s", request.Method, request.URL)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), "terradrift-issue root=abc123 fp=") {
				t.Fatalf("expected marker in patch body, got %q", body)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), scanReport); err != nil {
		t.Fatalf("upsert drift issue: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected list then patch, got %d calls", calls)
	}
}

func TestGitHubIssueNotifierClosesStaleFingerprintThenCreates(t *testing.T) {
	scanReport := report.DriftReport{
		RootID:          "abc123",
		Status:          report.ScanStatusDriftDetected,
		ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"}},
	}
	stale := githubIssueMarker("abc123", "cafebabe")
	calls := 0
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				payload := fmt.Sprintf(`[{"number":11,"body":%q}]`, stale)
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(payload))}, nil
			case 2:
				if request.Method != http.MethodPatch || request.URL.Path != "/repos/owner/repo/issues/11" {
					t.Fatalf("unexpected stale close: %s %s", request.Method, request.URL)
				}
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if !strings.Contains(string(body), `"state":"closed"`) {
					t.Fatalf("expected close payload, got %q", body)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{}"))}, nil
			case 3:
				if request.Method != http.MethodPost || request.URL.Path != "/repos/owner/repo/issues" {
					t.Fatalf("unexpected create: %s %s", request.Method, request.URL)
				}
				return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
			default:
				t.Fatalf("unexpected extra call %d: %s %s", calls, request.Method, request.URL)
				return nil, nil
			}
		}),
	}
	if err := notifier.Notify(context.Background(), scanReport); err != nil {
		t.Fatalf("replace stale issue: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected list, close stale, create, got %d calls", calls)
	}
}

func TestGitHubIssueNotifierClosesDuplicateMatches(t *testing.T) {
	scanReport := report.DriftReport{
		RootID:          "abc123",
		Status:          report.ScanStatusDriftDetected,
		ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"}},
	}
	marker := githubIssueMarker("abc123", githubIssueFPHash(scanReport))
	calls := 0
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			switch calls {
			case 1:
				payload := fmt.Sprintf(`[{"number":44,"body":%q},{"number":45,"body":%q}]`, marker, marker)
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(payload))}, nil
			case 2:
				if request.Method != http.MethodPatch || request.URL.Path != "/repos/owner/repo/issues/45" {
					t.Fatalf("expected duplicate close, got %s %s", request.Method, request.URL)
				}
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if !strings.Contains(string(body), `"state":"closed"`) {
					t.Fatalf("expected close payload, got %q", body)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{}"))}, nil
			case 3:
				if request.Method != http.MethodPatch || request.URL.Path != "/repos/owner/repo/issues/44" {
					t.Fatalf("expected upsert of first match, got %s %s", request.Method, request.URL)
				}
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if !strings.Contains(string(body), "terradrift-issue root=abc123 fp=") {
					t.Fatalf("expected marker in patch body, got %q", body)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{}"))}, nil
			default:
				t.Fatalf("unexpected extra call %d: %s %s", calls, request.Method, request.URL)
				return nil, nil
			}
		}),
	}
	if err := notifier.Notify(context.Background(), scanReport); err != nil {
		t.Fatalf("dedupe matching issues: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected list, close duplicate, upsert, got %d calls", calls)
	}
}

func TestGitHubIssueNotifierFailsClosedOnTooManyIssuePages(t *testing.T) {
	calls := 0
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet {
				t.Fatalf("expected list only, got %s %s", request.Method, request.URL)
			}
			header := make(http.Header)
			header.Set("Link", `<https://github.test/repos/owner/repo/issues?page=2>; rel="next"`)
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: header, Body: io.NopCloser(strings.NewReader("[]"))}, nil
		}),
	}
	err := notifier.Notify(context.Background(), report.DriftReport{RootID: "abc123", Status: report.ScanStatusDriftDetected})
	if err == nil || !strings.Contains(err.Error(), "too many open issues") {
		t.Fatalf("expected fail-closed list error, got %v", err)
	}
	if calls != githubIssueListMaxPages {
		t.Fatalf("listed %d pages, want %d", calls, githubIssueListMaxPages)
	}
}

func TestGitHubIssueNotifierSkipsPullRequests(t *testing.T) {
	scanReport := report.DriftReport{
		RootID:          "abc123",
		Status:          report.ScanStatusDriftDetected,
		ResourceChanges: []report.ResourceChange{{Address: "aws_instance.web", Actions: []string{"update"}, RiskLevel: "medium"}},
	}
	marker := githubIssueMarker("abc123", githubIssueFPHash(scanReport))
	calls := 0
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				payload := fmt.Sprintf(`[{"number":7,"body":%q,"pull_request":{"url":"https://github.test/pr/7"}}]`, marker)
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(payload))}, nil
			}
			if request.Method != http.MethodPost || request.URL.Path != "/repos/owner/repo/issues" {
				t.Fatalf("expected create after skipping PR, got %s %s", request.Method, request.URL)
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), scanReport); err != nil {
		t.Fatalf("skip pull request: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected list then create, got %d calls", calls)
	}
}

func TestGitHubIssueNotifierCloseResolved(t *testing.T) {
	calls := 0
	notifier := GitHubIssueNotifier{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				body := `[{"number":44,"body":"<!-- terradrift-issue root=abc123 fp=deadbeef -->"},{"number":9,"body":"unrelated"}]`
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}, nil
			}
			if request.Method != http.MethodPatch || request.URL.Path != "/repos/owner/repo/issues/44" {
				t.Fatalf("unexpected close: %s %s", request.Method, request.URL)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), `"state":"closed"`) {
				t.Fatalf("expected close payload, got %q", body)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.CloseResolved(context.Background(), report.DriftReport{RootID: "abc123", Status: report.ScanStatusNoDrift}); err != nil {
		t.Fatalf("close resolved issue: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected list then close, got %d calls", calls)
	}
}

func TestGitHubHTTPClientUsesSecureTimeout(t *testing.T) {
	client, err := githubHTTPClient(nil, githubAPIURL)
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

func TestGitHubAPIURL(t *testing.T) {
	got, err := GitHubAPIURL("")
	if err != nil || got != githubAPIURL {
		t.Fatalf("empty = %q, %v", got, err)
	}
	got, err = GitHubAPIURL("https://ghes.example.test/api/v3")
	if err != nil || got != "https://ghes.example.test/api/v3" {
		t.Fatalf("ghes = %q, %v", got, err)
	}
	for _, raw := range []string{
		"http://ghes.example.test/api/v3",
		"https://user:pass@ghes.example.test/api/v3",
		"https://ghes.example.test/api/v3?token=1",
		"https://127.0.0.1/api/v3",
		"https://10.0.0.1/api/v3",
		"https://localhost/api/v3",
	} {
		_, err = GitHubAPIURL(raw)
		if err == nil {
			t.Fatalf("expected reject %q", raw)
		}
		if strings.Contains(err.Error(), "pass") || strings.Contains(err.Error(), "token=1") {
			t.Fatalf("error echoed secret material: %v", err)
		}
	}
}

func TestGitHubPRNotifierHonorsGITHUBAPIURL(t *testing.T) {
	t.Setenv("GITHUB_API_URL", "https://ghes.example.test/api/v3")
	var got string
	notifier := GitHubPRNotifier{
		Repository: "owner/repo",
		Number:     12,
		Token:      "secret-token",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			got = request.URL.String()
			if request.Method == http.MethodGet {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("[]"))}, nil
			}
			return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}),
	}
	if err := notifier.Notify(context.Background(), report.DriftReport{TotalChangedResources: 1}); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if !strings.HasPrefix(got, "https://ghes.example.test/api/v3/repos/owner/repo/") {
		t.Fatalf("expected GHES API URL, got %q", got)
	}
}

func TestGitHubHTTPClientAllowsPrivateDialForEnterpriseHost(t *testing.T) {
	if githubAPIAllowHost(githubAPIURL) != "" {
		t.Fatal("public api.github.com must not skip private-IP blocking")
	}
	if githubAPIAllowHost("https://ghes.example.test/api/v3") != "ghes.example.test" {
		t.Fatal("expected GHES host to be the dialer allow-list")
	}
	if !allowPrivateWebhookIP("ghes.example.test", "ghes.example.test") {
		t.Fatal("expected matching GHES host to allow private IPs")
	}
	if allowPrivateWebhookIP("evil.test", "ghes.example.test") {
		t.Fatal("expected a different host to stay blocked")
	}
}
