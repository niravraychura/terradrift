// Package notify provides secret-safe notification integrations.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/niravraychura/terradrift/internal/redact"
	"github.com/niravraychura/terradrift/internal/report"
)

// HTTPDoer is the subset of http.Client needed by SlackNotifier.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SlackNotifier sends drift summaries to a Slack incoming webhook.
type SlackNotifier struct {
	WebhookURL string
	Client     HTTPDoer
}

// Notify sends a concise, redacted scan summary to Slack.
func (notifier SlackNotifier) Notify(ctx context.Context, scanReport report.DriftReport) error {
	webhookURL, err := validateGenericWebhookURL(notifier.WebhookURL)
	if err != nil {
		return err
	}
	client := notifier.Client
	if client == nil {
		client = secureWebhookClient()
	}

	payload := slackPayload{Text: RedactedNotificationMessage(scanReport)}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Slack notification: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Slack notification request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send Slack notification to %s: %w", redact.String(webhookURL), err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = closeResponseBody(response.Body)
		return fmt.Errorf("send Slack notification to %s: unexpected status %s", redact.String(webhookURL), response.Status)
	}
	if err := closeResponseBody(response.Body); err != nil {
		return fmt.Errorf("close Slack notification response: %w", err)
	}
	return nil
}

// RedactedNotificationMessage formats a scan summary without leaking raw local paths.
func RedactedNotificationMessage(scanReport report.DriftReport) string {
	return fmt.Sprintf("Terraform drift scan completed\nScan ID: %s\nStatus: %s\nResources checked: %d\nChanged resources: %d",
		scanReport.ScanID,
		scanReport.Status,
		scanReport.TotalResourcesChecked,
		scanReport.TotalChangedResources,
	)
}

type slackPayload struct {
	Text string `json:"text"`
}
