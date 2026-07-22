package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/niravraychura/terradrift/internal/report"
)

func TestWebhookNotifierSendsRedactedPayload(t *testing.T) {
	var body string
	notifier := WebhookNotifier{
		WebhookURL: "https://alerts.example.com/terradrift?token=secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected JSON content type, got %q", got)
			}
			data, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			body = string(data)
			return &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{
		Status:                report.ScanStatusDriftDetected,
		Directory:             "/secret/local/path",
		TotalResourcesChecked: 8,
		TotalChangedResources: 3,
	})
	if err != nil {
		t.Fatalf("expected webhook notification to succeed: %v", err)
	}
	if strings.Contains(body, "/secret/local/path") || strings.Contains(body, "secret-value") {
		t.Fatalf("expected webhook body to avoid path and URL secrets, got %q", body)
	}
	var payload webhookPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("expected webhook JSON, got %v: %q", err, body)
	}
	if payload.Status != report.ScanStatusDriftDetected || payload.TotalChangedResources != 3 || payload.TotalResourcesChecked != 8 {
		t.Fatalf("unexpected webhook payload: %#v", payload)
	}
}

func TestWebhookNotifierRejectsUnsafeURLs(t *testing.T) {
	for _, webhookURL := range []string{
		"",
		"http://alerts.example.com/terradrift",
		"https://user:pass@alerts.example.com/terradrift",
		"https://localhost/terradrift",
		"https://127.0.0.1/terradrift",
		"https://10.0.0.1/terradrift",
		"https://169.254.169.254/latest/meta-data",
	} {
		t.Run(webhookURL, func(t *testing.T) {
			err := WebhookNotifier{WebhookURL: webhookURL}.Notify(context.Background(), report.DriftReport{})
			if err == nil {
				t.Fatal("expected unsafe webhook URL to be rejected")
			}
		})
	}
}

func TestWebhookNotifierRedactsURLInErrors(t *testing.T) {
	notifier := WebhookNotifier{
		WebhookURL: "https://alerts.example.com/terradrift?token=secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Body: io.NopCloser(strings.NewReader("no"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{})
	if err == nil {
		t.Fatal("expected webhook status error")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected webhook URL query secret to be redacted from error, got %v", err)
	}
}
