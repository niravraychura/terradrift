// Package notify provides secret-safe notification integrations.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	CACertPath string
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
		client, err = secureWebhookClientFromCA(notifier.CACertPath)
		if err != nil {
			return err
		}
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

// RedactedNotificationMessage formats a scan summary without leaking raw local paths
// or attribute values. When drift is present, include the top five resource addresses
// and risk levels for actionable triage.
func RedactedNotificationMessage(scanReport report.DriftReport) string {
	message := fmt.Sprintf("Terraform scan completed\nScan ID: %s\nStatus: %s\nPlan mode: %s\nResources checked: %d\nChanged resources: %d",
		scanReport.ScanID,
		scanReport.Status,
		scanReport.PlanMode,
		scanReport.TotalResourcesChecked,
		scanReport.TotalChangedResources,
	)
	if len(scanReport.ResourceChanges) == 0 {
		return message
	}
	limit := 5
	if len(scanReport.ResourceChanges) < limit {
		limit = len(scanReport.ResourceChanges)
	}
	message += "\nTop changes:"
	for _, change := range scanReport.ResourceChanges[:limit] {
		risk := strings.TrimSpace(change.RiskLevel)
		if risk == "" {
			risk = "unknown"
		}
		message += fmt.Sprintf("\n- %s (%s)", change.Address, risk)
	}
	if len(scanReport.ResourceChanges) > limit {
		message += fmt.Sprintf("\n- … and %d more", len(scanReport.ResourceChanges)-limit)
	}
	return message
}

type slackPayload struct {
	Text string `json:"text"`
}
