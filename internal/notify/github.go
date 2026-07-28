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

const githubAPIURL = "https://api.github.com"

// GitHubPRNotifier posts a scan summary to a pull request comment thread.
type GitHubPRNotifier struct {
	Repository string
	Number     int
	Token      string
	Client     HTTPDoer
	APIURL     string
}

// Notify posts a concise, redacted drift summary to the configured pull request.
func (notifier GitHubPRNotifier) Notify(ctx context.Context, scanReport report.DriftReport) error {
	repository := strings.Trim(strings.TrimSpace(notifier.Repository), "/")
	if len(strings.Split(repository, "/")) != 2 || strings.Contains(repository, " ") {
		return fmt.Errorf("GitHub repository must be owner/repo")
	}
	if notifier.Number <= 0 {
		return fmt.Errorf("GitHub pull request number must be greater than zero")
	}
	token := strings.TrimSpace(notifier.Token)
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is required for GitHub pull request summaries")
	}
	apiURL := strings.TrimRight(notifier.APIURL, "/")
	if apiURL == "" {
		apiURL = githubAPIURL
	}
	body, err := json.Marshal(struct {
		Body string `json:"body"`
	}{Body: "## TerraDrift Scan\n\n" + RedactedNotificationMessage(scanReport)})
	if err != nil {
		return fmt.Errorf("encode GitHub pull request summary: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/issues/%d/comments", apiURL, repository, notifier.Number), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create GitHub pull request summary: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "terradrift")
	request.Header.Set("Content-Type", "application/json")
	client := notifier.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send GitHub pull request summary to %s: %w", redact.String(repository), err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("send GitHub pull request summary to %s: unexpected status %s", redact.String(repository), response.Status)
	}
	return nil
}

// GitHubIssueNotifier creates an issue for persistent drift.
type GitHubIssueNotifier struct {
	Repository string
	Token      string
	Client     HTTPDoer
	APIURL     string
}

// Notify creates a concise persistent-drift issue.
func (notifier GitHubIssueNotifier) Notify(ctx context.Context, scanReport report.DriftReport) error {
	repository := strings.Trim(strings.TrimSpace(notifier.Repository), "/")
	if len(strings.Split(repository, "/")) != 2 || strings.Contains(repository, " ") {
		return fmt.Errorf("GitHub repository must be owner/repo")
	}
	token := strings.TrimSpace(notifier.Token)
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is required for GitHub drift issues")
	}
	apiURL := strings.TrimRight(notifier.APIURL, "/")
	if apiURL == "" {
		apiURL = githubAPIURL
	}
	body, err := json.Marshal(struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{Title: "TerraDrift: persistent drift detected", Body: "## Persistent TerraDrift Drift\n\n" + RedactedNotificationMessage(scanReport)})
	if err != nil {
		return fmt.Errorf("encode GitHub drift issue: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/issues", apiURL, repository), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create GitHub drift issue: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "terradrift")
	request.Header.Set("Content-Type", "application/json")
	client := notifier.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("create GitHub drift issue in %s: %w", redact.String(repository), err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("create GitHub drift issue in %s: unexpected status %s", redact.String(repository), response.Status)
	}
	return nil
}
