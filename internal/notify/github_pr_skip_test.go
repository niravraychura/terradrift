package notify

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPRPathTouchesRoot(t *testing.T) {
	tests := []struct {
		file string
		root string
		want bool
	}{
		{file: "terraform/prod/main.tf", root: "terraform/prod", want: true},
		{file: "terraform/prod", root: "terraform/prod", want: true},
		{file: "terraform/production/main.tf", root: "terraform/prod", want: false},
		{file: "README.md", root: "terraform/prod", want: false},
		{file: "README.md", root: "", want: true},
		{file: "README.md", root: ".", want: true},
		{file: "../secret", root: "terraform/prod", want: false},
		{file: "", root: "terraform/prod", want: false},
	}
	for _, test := range tests {
		if got := prPathTouchesRoot(test.file, test.root); got != test.want {
			t.Fatalf("prPathTouchesRoot(%q, %q) = %v, want %v", test.file, test.root, got, test.want)
		}
	}
}

func TestGitHubOpenPRSkipperMatchesRoot(t *testing.T) {
	calls := 0
	skipper := &GitHubOpenPRSkipper{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			switch {
			case request.URL.Path == "/repos/owner/repo/pulls":
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`[{"number":12},{"number":13}]`))}, nil
			case request.URL.Path == "/repos/owner/repo/pulls/12/files":
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`[{"filename":"README.md"}]`))}, nil
			case request.URL.Path == "/repos/owner/repo/pulls/13/files":
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`[{"filename":"terraform/prod/main.tf","previous_filename":"terraform/old/main.tf"}]`))}, nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL)
				return nil, nil
			}
		}),
	}
	number, err := skipper.MatchingPR(context.Background(), "terraform/prod")
	if err != nil || number != 13 {
		t.Fatalf("MatchingPR = %d, %v", number, err)
	}
	number, err = skipper.MatchingPR(context.Background(), "modules/vpc")
	if err != nil || number != 0 {
		t.Fatalf("expected no match, got %d, %v", number, err)
	}
	if calls != 3 {
		t.Fatalf("expected cached PR list, got %d calls", calls)
	}
}

func TestGitHubOpenPRSkipperFailsClosedOnTooManyPages(t *testing.T) {
	skipper := &GitHubOpenPRSkipper{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			header := make(http.Header)
			header.Set("Link", `<https://github.test/repos/owner/repo/pulls?page=2>; rel="next"`)
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: header, Body: io.NopCloser(strings.NewReader("[]"))}, nil
		}),
	}
	_, err := skipper.MatchingPR(context.Background(), "terraform/prod")
	if err == nil || !strings.Contains(err.Error(), "too many open pull requests") {
		t.Fatalf("expected fail-closed list error, got %v", err)
	}
}

func TestRepoRelativeRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "terraform", "prod")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rel, err := RepoRelativeRoot(root, dir)
	if err != nil || rel != "terraform/prod" {
		t.Fatalf("RepoRelativeRoot = %q, %v", rel, err)
	}
	if _, err := RepoRelativeRoot(dir, root); err == nil {
		t.Fatal("expected directory outside workspace root to fail")
	}
}
