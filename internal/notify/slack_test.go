package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestSlackNotifierSendsRedactedSummary(t *testing.T) {
	var body string
	notifier := SlackNotifier{
		WebhookURL: "https://hooks.slack.com/services/T000/B000/secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			data, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			body = string(data)
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{
		Status:                report.ScanStatusDriftDetected,
		Directory:             "/secret/local/path",
		TotalResourcesChecked: 10,
		TotalChangedResources: 2,
	})
	if err != nil {
		t.Fatalf("expected Slack notification to succeed: %v", err)
	}
	if strings.Contains(body, "/secret/local/path") || strings.Contains(body, "secret-value") {
		t.Fatalf("expected Slack body to avoid path and webhook secrets, got %q", body)
	}
	if !strings.Contains(body, "drift_detected") || !strings.Contains(body, "Changed resources: 2") {
		t.Fatalf("expected useful summary, got %q", body)
	}
}

func TestSlackNotifierRedactsWebhookURLInErrors(t *testing.T) {
	notifier := SlackNotifier{
		WebhookURL: "https://hooks.slack.com/services/T000/B000/secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Body: io.NopCloser(strings.NewReader("no"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{})
	if err == nil {
		t.Fatal("expected Slack status error")
	}
	if strings.Contains(err.Error(), "secret-value") || strings.Contains(err.Error(), "/services/") {
		t.Fatalf("expected webhook URL to be redacted from error, got %v", err)
	}
}
