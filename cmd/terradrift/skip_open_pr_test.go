package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/notify"
	"github.com/niravraychura/terradrift/internal/report"
)

type skipRoundTrip func(*http.Request) (*http.Response, error)

func (fn skipRoundTrip) Do(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSkipOpenPRReportDoesNotLookLikeNoDrift(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "terraform", "prod")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	skipper := &notify.GitHubOpenPRSkipper{
		Repository: "owner/repo",
		Token:      "secret-token",
		APIURL:     "https://github.test",
		Client: skipRoundTrip(func(request *http.Request) (*http.Response, error) {
			if strings.HasSuffix(request.URL.Path, "/pulls") {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`[{"number":7}]`))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`[{"filename":"terraform/prod/main.tf"}]`))}, nil
		}),
	}
	var stderr bytes.Buffer
	scanReport, skipped, err := skipOpenPRReport(context.Background(), skipper, root, dir, false, &stderr)
	if err != nil || !skipped {
		t.Fatalf("skipOpenPRReport = %#v, %v, %v", scanReport, skipped, err)
	}
	if scanReport.Status != report.ScanStatusSkipped || scanReport.Status == report.ScanStatusNoDrift {
		t.Fatalf("status = %q", scanReport.Status)
	}
	if !strings.Contains(stderr.String(), "open pull request 7") {
		t.Fatalf("expected skip reason, got %q", stderr.String())
	}
}
