package notify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	githubAPIURL            = "https://api.github.com"
	githubHTTPTimeout       = 30 * time.Second
	githubPRCommentMarker   = "<!-- terradrift-pr-comment -->"
	githubPRCommentHeading  = "## TerraDrift Scan"
	githubIssueListMaxPages = 5
)

// GitHubPRNotifier posts a scan summary to a pull request comment thread.
type GitHubPRNotifier struct {
	Repository string
	Number     int
	Token      string
	Client     HTTPDoer
	APIURL     string
}

// Notify upserts a concise, redacted drift summary on the configured pull request.
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
	client, err := githubHTTPClient(notifier.Client)
	if err != nil {
		return err
	}
	commentID, err := findTerraDriftPRCommentID(ctx, client, token, fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100", apiURL, repository, notifier.Number), repository)
	if err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		Body string `json:"body"`
	}{Body: githubPRCommentMarker + "\n" + githubPRCommentHeading + "\n\n" + RedactedNotificationMessage(scanReport)})
	if err != nil {
		return fmt.Errorf("encode GitHub pull request summary: %w", err)
	}
	method := http.MethodPost
	target := fmt.Sprintf("%s/repos/%s/issues/%d/comments", apiURL, repository, notifier.Number)
	if commentID != 0 {
		method = http.MethodPatch
		target = fmt.Sprintf("%s/repos/%s/issues/comments/%d", apiURL, repository, commentID)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create GitHub pull request summary: %w", err)
	}
	setGitHubJSONHeaders(request, token)
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

func findTerraDriftPRCommentID(ctx context.Context, client HTTPDoer, token, listURL, repository string) (int64, error) {
	var found int64
	currentURL := listURL
	for currentURL != "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return 0, fmt.Errorf("list GitHub pull request comments: %w", err)
		}
		setGitHubJSONHeaders(request, token)
		response, err := client.Do(request)
		if err != nil {
			return 0, fmt.Errorf("list GitHub pull request comments on %s: %s", redact.String(repository), redact.String(err.Error()))
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			_ = closeResponseBody(response.Body)
			return 0, fmt.Errorf("list GitHub pull request comments on %s: unexpected status %s", redact.String(repository), response.Status)
		}
		var comments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(response.Body).Decode(&comments); err != nil {
			_ = closeResponseBody(response.Body)
			return 0, fmt.Errorf("decode GitHub pull request comments: %w", err)
		}
		for _, comment := range comments {
			if strings.Contains(comment.Body, githubPRCommentMarker) || strings.Contains(comment.Body, githubPRCommentHeading) {
				found = comment.ID
			}
		}
		currentURL = parseGitHubNextLink(response.Header.Get("Link"))
		_ = closeResponseBody(response.Body)
	}
	return found, nil
}

func parseGitHubNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		parts := strings.Split(strings.TrimSpace(link), ";")
		if len(parts) != 2 {
			continue
		}
		if strings.Contains(parts[1], `rel="next"`) {
			url := strings.TrimSpace(parts[0])
			if len(url) > 2 && url[0] == '<' && url[len(url)-1] == '>' {
				return url[1 : len(url)-1]
			}
		}
	}
	return ""
}

func setGitHubJSONHeaders(request *http.Request, token string) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "terradrift")
	request.Header.Set("Content-Type", "application/json")
}

// GitHubIssueNotifier upserts a persistent-drift issue keyed by root and fingerprint.
type GitHubIssueNotifier struct {
	Repository string
	Token      string
	Client     HTTPDoer
	APIURL     string
	Labels     []string
}

type githubIssue struct {
	Number      int    `json:"number"`
	Body        string `json:"body"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
}

func githubIssueFPHash(scanReport report.DriftReport) string {
	sum := sha256.Sum256([]byte(report.DriftFingerprint(scanReport)))
	return hex.EncodeToString(sum[:8])
}

func githubIssueMarker(rootID, fpHash string) string {
	return fmt.Sprintf("<!-- terradrift-issue root=%s fp=%s -->", rootID, fpHash)
}

func parseGitHubIssueMarker(body string) (rootID, fpHash string, ok bool) {
	const prefix = "<!-- terradrift-issue root="
	i := strings.Index(body, prefix)
	if i < 0 {
		return "", "", false
	}
	rest := body[i+len(prefix):]
	space := strings.IndexByte(rest, ' ')
	if space < 0 {
		return "", "", false
	}
	rootID = rest[:space]
	rest = rest[space+1:]
	if !strings.HasPrefix(rest, "fp=") {
		return "", "", false
	}
	rest = strings.TrimPrefix(rest, "fp=")
	end := strings.Index(rest, " -->")
	if end < 0 {
		return "", "", false
	}
	fpHash = rest[:end]
	if rootID == "" || fpHash == "" {
		return "", "", false
	}
	return rootID, fpHash, true
}

func githubIssueBody(scanReport report.DriftReport, rootID, fpHash string) string {
	return githubIssueMarker(rootID, fpHash) + "\n## Persistent TerraDrift Drift\n\n" + RedactedNotificationMessage(scanReport)
}

// Notify creates or updates one issue for this root+fingerprint and closes stale same-root issues.
func (notifier GitHubIssueNotifier) Notify(ctx context.Context, scanReport report.DriftReport) error {
	repository, token, client, apiURL, err := notifier.ready()
	if err != nil {
		return err
	}
	if err := validation.GitHubIssueLabels(notifier.Labels); err != nil {
		return err
	}
	rootID := strings.TrimSpace(scanReport.RootID)
	if rootID == "" {
		return validation.New("GitHub drift issue", errors.New("root_id is required"))
	}
	fpHash := githubIssueFPHash(scanReport)
	issues, err := listOpenGitHubIssues(ctx, client, token, apiURL, repository)
	if err != nil {
		return err
	}
	var match int
	for _, issue := range issues {
		issueRoot, issueFP, ok := parseGitHubIssueMarker(issue.Body)
		if !ok || issueRoot != rootID {
			continue
		}
		if issueFP == fpHash {
			if match != 0 {
				if err := patchGitHubIssue(ctx, client, token, apiURL, repository, issue.Number, map[string]any{"state": "closed"}); err != nil {
					return err
				}
				continue
			}
			match = issue.Number
			continue
		}
		if err := patchGitHubIssue(ctx, client, token, apiURL, repository, issue.Number, map[string]any{"state": "closed"}); err != nil {
			return err
		}
	}
	body := githubIssueBody(scanReport, rootID, fpHash)
	if match != 0 {
		return patchGitHubIssue(ctx, client, token, apiURL, repository, match, map[string]any{"body": body})
	}
	payload := struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels,omitempty"`
	}{Title: "TerraDrift: persistent drift detected", Body: body, Labels: notifier.Labels}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode GitHub drift issue: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/issues", apiURL, repository), bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create GitHub drift issue: %w", err)
	}
	setGitHubJSONHeaders(request, token)
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

// CloseResolved closes open TerraDrift issues for this root after a clean scan.
func (notifier GitHubIssueNotifier) CloseResolved(ctx context.Context, scanReport report.DriftReport) error {
	repository, token, client, apiURL, err := notifier.ready()
	if err != nil {
		return err
	}
	rootID := strings.TrimSpace(scanReport.RootID)
	if rootID == "" {
		return validation.New("GitHub drift issue", errors.New("root_id is required"))
	}
	issues, err := listOpenGitHubIssues(ctx, client, token, apiURL, repository)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		issueRoot, _, ok := parseGitHubIssueMarker(issue.Body)
		if !ok || issueRoot != rootID {
			continue
		}
		if err := patchGitHubIssue(ctx, client, token, apiURL, repository, issue.Number, map[string]any{"state": "closed"}); err != nil {
			return err
		}
	}
	return nil
}

func (notifier GitHubIssueNotifier) ready() (repository, token string, client HTTPDoer, apiURL string, err error) {
	repository, token, err = validateGitHubNotifier(notifier.Repository, notifier.Token)
	if err != nil {
		return "", "", nil, "", err
	}
	apiURL = strings.TrimRight(notifier.APIURL, "/")
	if apiURL == "" {
		apiURL = githubAPIURL
	}
	client, err = githubHTTPClient(notifier.Client)
	if err != nil {
		return "", "", nil, "", err
	}
	return repository, token, client, apiURL, nil
}

func listOpenGitHubIssues(ctx context.Context, client HTTPDoer, token, apiURL, repository string) ([]githubIssue, error) {
	var found []githubIssue
	currentURL := fmt.Sprintf("%s/repos/%s/issues?state=open&per_page=100", apiURL, repository)
	for page := 0; currentURL != "" && page < githubIssueListMaxPages; page++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("list GitHub issues: %w", err)
		}
		setGitHubJSONHeaders(request, token)
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("list GitHub issues on %s: %s", redact.String(repository), redact.String(err.Error()))
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			_ = closeResponseBody(response.Body)
			return nil, fmt.Errorf("list GitHub issues on %s: unexpected status %s", redact.String(repository), response.Status)
		}
		var issues []githubIssue
		if err := json.NewDecoder(response.Body).Decode(&issues); err != nil {
			_ = closeResponseBody(response.Body)
			return nil, fmt.Errorf("decode GitHub issues: %w", err)
		}
		for _, issue := range issues {
			if issue.PullRequest != nil {
				continue
			}
			found = append(found, issue)
		}
		currentURL = parseGitHubNextLink(response.Header.Get("Link"))
		_ = closeResponseBody(response.Body)
	}
	if currentURL != "" {
		return nil, fmt.Errorf("list GitHub issues on %s: too many open issues to search safely", redact.String(repository))
	}
	return found, nil
}

func patchGitHubIssue(ctx context.Context, client HTTPDoer, token, apiURL, repository string, number int, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode GitHub issue update: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/%d", apiURL, repository, number), bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("update GitHub issue: %w", err)
	}
	setGitHubJSONHeaders(request, token)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("update GitHub issue in %s: %s", redact.String(repository), redact.String(err.Error()))
	}
	defer func() { _ = closeResponseBody(response.Body) }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("update GitHub issue in %s: unexpected status %s", redact.String(repository), response.Status)
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
