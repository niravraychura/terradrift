package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/niravraychura/terradrift/internal/redact"
)

type openPRFiles struct {
	Number int
	Paths  []string
}

// GitHubOpenPRSkipper finds open pull requests whose files touch a Terraform root.
type GitHubOpenPRSkipper struct {
	Repository string
	Token      string
	Client     HTTPDoer
	APIURL     string

	mu    sync.Mutex
	cache []openPRFiles
}

// MatchingPR returns the first open PR that touches rootRel (slash path, empty = repo root).
func (skipper *GitHubOpenPRSkipper) MatchingPR(ctx context.Context, rootRel string) (int, error) {
	prs, err := skipper.openPRs(ctx)
	if err != nil {
		return 0, err
	}
	for _, pr := range prs {
		for _, filePath := range pr.Paths {
			if prPathTouchesRoot(filePath, rootRel) {
				return pr.Number, nil
			}
		}
	}
	return 0, nil
}

func (skipper *GitHubOpenPRSkipper) openPRs(ctx context.Context) ([]openPRFiles, error) {
	skipper.mu.Lock()
	defer skipper.mu.Unlock()
	if skipper.cache != nil {
		return skipper.cache, nil
	}
	repository, token, err := validateGitHubNotifier(skipper.Repository, skipper.Token)
	if err != nil {
		return nil, err
	}
	apiURL, err := githubAPIBase(skipper.APIURL)
	if err != nil {
		return nil, err
	}
	client, err := githubHTTPClient(skipper.Client, apiURL)
	if err != nil {
		return nil, err
	}
	prs, err := listOpenPullRequests(ctx, client, token, apiURL, repository)
	if err != nil {
		return nil, err
	}
	cache := make([]openPRFiles, 0, len(prs))
	for _, number := range prs {
		paths, err := listPullRequestFiles(ctx, client, token, apiURL, repository, number)
		if err != nil {
			return nil, err
		}
		cache = append(cache, openPRFiles{Number: number, Paths: paths})
	}
	skipper.cache = cache
	return cache, nil
}

func listOpenPullRequests(ctx context.Context, client HTTPDoer, token, apiURL, repository string) ([]int, error) {
	var numbers []int
	currentURL := fmt.Sprintf("%s/repos/%s/pulls?state=open&per_page=100", apiURL, repository)
	for page := 0; currentURL != "" && page < githubIssueListMaxPages; page++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("list GitHub pull requests: %w", err)
		}
		setGitHubJSONHeaders(request, token)
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("list GitHub pull requests on %s: %s", redact.String(repository), redact.String(err.Error()))
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			_ = closeResponseBody(response.Body)
			return nil, fmt.Errorf("list GitHub pull requests on %s: unexpected status %s", redact.String(repository), response.Status)
		}
		var pulls []struct {
			Number int `json:"number"`
		}
		if err := json.NewDecoder(response.Body).Decode(&pulls); err != nil {
			_ = closeResponseBody(response.Body)
			return nil, fmt.Errorf("decode GitHub pull requests: %w", err)
		}
		for _, pull := range pulls {
			if pull.Number > 0 {
				numbers = append(numbers, pull.Number)
			}
		}
		currentURL = parseGitHubNextLink(response.Header.Get("Link"))
		_ = closeResponseBody(response.Body)
	}
	if currentURL != "" {
		return nil, fmt.Errorf("list GitHub pull requests on %s: too many open pull requests to search safely", redact.String(repository))
	}
	return numbers, nil
}

func listPullRequestFiles(ctx context.Context, client HTTPDoer, token, apiURL, repository string, number int) ([]string, error) {
	var paths []string
	currentURL := fmt.Sprintf("%s/repos/%s/pulls/%d/files?per_page=100", apiURL, repository, number)
	for page := 0; currentURL != "" && page < githubIssueListMaxPages; page++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("list GitHub pull request files: %w", err)
		}
		setGitHubJSONHeaders(request, token)
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("list GitHub pull request files on %s: %s", redact.String(repository), redact.String(err.Error()))
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			_ = closeResponseBody(response.Body)
			return nil, fmt.Errorf("list GitHub pull request files on %s: unexpected status %s", redact.String(repository), response.Status)
		}
		var files []struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
		}
		if err := json.NewDecoder(response.Body).Decode(&files); err != nil {
			_ = closeResponseBody(response.Body)
			return nil, fmt.Errorf("decode GitHub pull request files: %w", err)
		}
		for _, file := range files {
			if file.Filename != "" {
				paths = append(paths, file.Filename)
			}
			if file.PreviousFilename != "" {
				paths = append(paths, file.PreviousFilename)
			}
		}
		currentURL = parseGitHubNextLink(response.Header.Get("Link"))
		_ = closeResponseBody(response.Body)
	}
	if currentURL != "" {
		return nil, fmt.Errorf("list GitHub pull request files on %s: too many files to search safely", redact.String(repository))
	}
	return paths, nil
}

func prPathTouchesRoot(filePath, rootRel string) bool {
	filePath = strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	if filePath == "" {
		return false
	}
	filePath = path.Clean(filePath)
	if filePath == ".." || strings.HasPrefix(filePath, "../") {
		return false
	}
	rootRel = strings.Trim(strings.ReplaceAll(rootRel, "\\", "/"), "/")
	if rootRel == "" || rootRel == "." {
		return true
	}
	rootRel = path.Clean(rootRel)
	return filePath == rootRel || strings.HasPrefix(filePath, rootRel+"/")
}

// RepoRelativeRoot maps a Terraform directory onto GitHub pull-request file paths.
func RepoRelativeRoot(workspaceRoot, directory string) (string, error) {
	absDir := directory
	if !filepath.IsAbs(absDir) {
		resolved, err := filepath.Abs(absDir)
		if err != nil {
			return "", fmt.Errorf("resolve terraform directory: %w", err)
		}
		absDir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = resolved
	}
	base := strings.TrimSpace(workspaceRoot)
	if base == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve workspace root: %w", err)
		}
		base = cwd
	}
	if !filepath.IsAbs(base) {
		resolved, err := filepath.Abs(base)
		if err != nil {
			return "", fmt.Errorf("resolve workspace root: %w", err)
		}
		base = resolved
	}
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}
	rel, err := filepath.Rel(base, absDir)
	if err != nil {
		return "", fmt.Errorf("terraform directory is outside workspace root")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("terraform directory is outside workspace root")
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "", nil
	}
	return rel, nil
}
