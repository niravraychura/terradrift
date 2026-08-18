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

func TestTeamsNotifierSendsRedactedConnectorCard(t *testing.T) {
	var body string
	notifier := TeamsNotifier{
		WebhookURL: "https://example.webhook.office.com/webhookb2/secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected JSON content type, got %q", got)
			}
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
		t.Fatalf("expected Teams notification to succeed: %v", err)
	}
	if strings.Contains(body, "/secret/local/path") || strings.Contains(body, "secret-value") {
		t.Fatalf("expected Teams body to avoid path and webhook secrets, got %q", body)
	}
	var payload teamsPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("expected connector card JSON, got %v: %q", err, body)
	}
	if payload.Type != "MessageCard" || payload.Context == "" || payload.ThemeColor == "" {
		t.Fatalf("expected Teams connector card fields, got %#v", payload)
	}
	if !strings.Contains(payload.Text, "drift_detected") || !strings.Contains(payload.Text, "Changed resources: 2") {
		t.Fatalf("expected useful summary, got %#v", payload)
	}
}

func TestTeamsNotifierRedactsWebhookURLInErrors(t *testing.T) {
	notifier := TeamsNotifier{
		WebhookURL: "https://example.webhook.office.com/webhookb2/secret-value",
		Client: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Body: io.NopCloser(strings.NewReader("no"))}, nil
		}),
	}

	err := notifier.Notify(context.Background(), report.DriftReport{})
	if err == nil {
		t.Fatal("expected Teams status error")
	}
	if strings.Contains(err.Error(), "secret-value") || strings.Contains(err.Error(), "webhookb2") {
		t.Fatalf("expected webhook URL to be redacted from error, got %v", err)
	}
}
