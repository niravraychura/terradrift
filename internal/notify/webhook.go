package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/niravraychura/terradrift/internal/redact"
	"github.com/niravraychura/terradrift/internal/report"
)

// WebhookNotifier sends drift summaries to a generic HTTPS webhook endpoint.
type WebhookNotifier struct {
	WebhookURL string
	Client     HTTPDoer
}

// Notify sends a minimal, redacted JSON payload to a generic HTTPS webhook.
func (notifier WebhookNotifier) Notify(ctx context.Context, scanReport report.DriftReport) error {
	webhookURL, err := validateGenericWebhookURL(notifier.WebhookURL)
	if err != nil {
		return err
	}
	client := notifier.Client
	if client == nil {
		client = http.DefaultClient
	}

	payload := webhookPayload{
		Status:                scanReport.Status,
		TotalResourcesChecked: scanReport.TotalResourcesChecked,
		TotalChangedResources: scanReport.TotalChangedResources,
		Message:               RedactedNotificationMessage(scanReport),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode webhook notification: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook notification request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "terradrift")

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send webhook notification to %s: %w", redact.String(webhookURL), err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = response.Body.Close()
		return fmt.Errorf("send webhook notification to %s: unexpected status %s", redact.String(webhookURL), response.Status)
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close webhook notification response: %w", err)
	}
	return nil
}

func validateGenericWebhookURL(rawURL string) (string, error) {
	webhookURL := strings.TrimSpace(rawURL)
	if webhookURL == "" {
		return "", fmt.Errorf("webhook URL is required")
	}
	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return "", fmt.Errorf("parse webhook URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("webhook URL must use https")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("webhook URL must not include user info")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("webhook URL host is required")
	}
	if isBlockedWebhookHost(host) {
		return "", fmt.Errorf("webhook URL host is not allowed")
	}
	return parsed.String(), nil
}

func isBlockedWebhookHost(host string) bool {
	normalizedHost := strings.TrimSuffix(strings.ToLower(host), ".")
	if normalizedHost == "localhost" || strings.HasSuffix(normalizedHost, ".localhost") {
		return true
	}
	ip := net.ParseIP(normalizedHost)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

type webhookPayload struct {
	Status                report.ScanStatus `json:"status"`
	TotalResourcesChecked int               `json:"total_resources_checked"`
	TotalChangedResources int               `json:"total_changed_resources"`
	Message               string            `json:"message"`
}
