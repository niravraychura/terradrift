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

// TeamsNotifier sends drift summaries to a Microsoft Teams incoming webhook.
type TeamsNotifier struct {
	WebhookURL string
	Client     HTTPDoer
}

// Notify sends a concise, redacted connector-card summary to Microsoft Teams.
func (notifier TeamsNotifier) Notify(ctx context.Context, scanReport report.DriftReport) error {
	webhookURL, err := validateGenericWebhookURL(notifier.WebhookURL)
	if err != nil {
		return err
	}
	client := notifier.Client
	if client == nil {
		client = secureWebhookClient()
	}

	payload := teamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: teamsThemeColor(scanReport.Status),
		Summary:    "Terraform scan completed",
		Title:      "Terraform scan completed",
		Text:       RedactedNotificationMessage(scanReport),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Teams notification: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Teams notification request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send Teams notification to %s: %w", redact.String(webhookURL), err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = closeResponseBody(response.Body)
		return fmt.Errorf("send Teams notification to %s: unexpected status %s", redact.String(webhookURL), response.Status)
	}
	if err := closeResponseBody(response.Body); err != nil {
		return fmt.Errorf("close Teams notification response: %w", err)
	}
	return nil
}

func teamsThemeColor(status report.ScanStatus) string {
	switch status {
	case report.ScanStatusDriftDetected, report.ScanStatusChangesDetected:
		return "d83b01"
	case report.ScanStatusFailed:
		return "a4262c"
	default:
		return "107c10"
	}
}

type teamsPayload struct {
	Type       string `json:"@type"`
	Context    string `json:"@context"`
	ThemeColor string `json:"themeColor"`
	Summary    string `json:"summary"`
	Title      string `json:"title"`
	Text       string `json:"text"`
}
