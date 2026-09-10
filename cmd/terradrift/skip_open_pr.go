package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/notify"
	"github.com/niravraychura/terradrift/internal/report"
	"github.com/niravraychura/terradrift/internal/scanner"
)

func skipOpenPRReport(ctx context.Context, skipper *notify.GitHubOpenPRSkipper, workspaceRoot, directory string, redactPaths bool, stderr io.Writer) (report.DriftReport, bool, error) {
	if skipper == nil {
		return report.DriftReport{}, false, nil
	}
	absDir, err := scanner.ValidateDirectory(directory)
	if err != nil {
		return report.DriftReport{}, false, err
	}
	base := workspaceRoot
	if strings.TrimSpace(base) != "" {
		base, err = scanner.ValidateDirectory(base)
		if err != nil {
			return report.DriftReport{}, false, err
		}
	}
	rootRel, err := notify.RepoRelativeRoot(base, absDir)
	if err != nil {
		return report.DriftReport{}, false, err
	}
	number, err := skipper.MatchingPR(ctx, rootRel)
	if err != nil {
		return report.DriftReport{}, false, err
	}
	if number == 0 {
		return report.DriftReport{}, false, nil
	}
	if stderr != nil {
		_, _ = fmt.Fprintf(stderr, "skipped: open pull request %d touches this root\n", number)
	}
	dir := absDir
	if redactPaths {
		dir = "[REDACTED]"
	}
	now := time.Now().UTC()
	return report.DriftReport{
		ScanID:          fmt.Sprintf("skipped-pr-%d", number),
		Status:          report.ScanStatusSkipped,
		Directory:       dir,
		ResourceChanges: []report.ResourceChange{},
		StartedAt:       now,
		CompletedAt:     now,
	}, true, nil
}

func newOpenPRSkipper(enabled bool, repository string) *notify.GitHubOpenPRSkipper {
	if !enabled {
		return nil
	}
	return &notify.GitHubOpenPRSkipper{Repository: repository, Token: os.Getenv("GITHUB_TOKEN")}
}
