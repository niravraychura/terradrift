package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/redact"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/validation"
)

const (
	githubAPIURL      = "https://api.github.com"
	githubHTTPTimeout = 30 * time.Second
)

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
	repository, token, err := validateGitHubNotifier(notifier.Repository, notifier.Token)
	if err != nil {
		return err
	}
	if notifier.Number <= 0 {
		return validation.New("GitHub pull request number", errors.New("must be greater than zero"))
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
	client, err := githubHTTPClient(notifier.Client)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send GitHub pull request summary to %s: %s", redact.String(repository), redact.String(err.Error()))
	}
	defer func() { _ = closeResponseBody(response.Body) }()
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
	repository, token, err := validateGitHubNotifier(notifier.Repository, notifier.Token)
	if err != nil {
		return err
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
	client, err := githubHTTPClient(notifier.Client)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("create GitHub drift issue in %s: %s", redact.String(repository), redact.String(err.Error()))
	}
	defer func() { _ = closeResponseBody(response.Body) }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("create GitHub drift issue in %s: unexpected status %s", redact.String(repository), response.Status)
	}
	return nil
}

func githubHTTPClient(client HTTPDoer) (HTTPDoer, error) {
	if client != nil {
		return client, nil
	}
	secure, err := secureWebhookClientFromCA("")
	if err != nil {
		return nil, fmt.Errorf("create GitHub HTTP client: %w", err)
	}
	secure.Timeout = githubHTTPTimeout
	return secure, nil
}

func validateGitHubNotifier(rawRepository string, rawToken string) (string, string, error) {
	repository := strings.Trim(strings.TrimSpace(rawRepository), "/")
	if len(strings.Split(repository, "/")) != 2 || strings.Contains(repository, " ") {
		return "", "", validation.New("GitHub repository", errors.New("must be owner/repo"))
	}
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", "", validation.New("GITHUB_TOKEN", errors.New("is required"))
	}
	return repository, token, nil
}
