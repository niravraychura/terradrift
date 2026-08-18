package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/notify"
	"github.com/niravraychura/terradrift/internal/policy"
	"github.com/niravraychura/terradrift/internal/report"
)

// deliveryOptions configures post-scan policy, persistence, and notification delivery.
type deliveryOptions struct {
	AttributeValues      bool
	ArtifactURL          string
	HistoryDir           string
	HistoryRetention     int
	HistoryCompressed    bool
	DashboardHTMLPath    string
	PolicyCommand        string
	PolicyArgs           []string
	NotifyTarget         string
	SlackWebhookURL      string
	TeamsWebhookURL      string
	WebhookURL           string
	WebhookCACert        string
	GitHubRepository     string
	GitHubPR             int
	GitHubIssueAfter     int
	OwnerWebhooks        map[string]string
	NotificationThrottle bool
	// historyMu serializes history/dashboard side effects for concurrent scan-all roots.
	historyMu *sync.Mutex
}

// persistableReport returns the report used for policy, history, artifacts, dashboards,
// and notifications. When AttributeValues is false, Before/After are cleared (paths remain).
func persistableReport(scanReport report.DriftReport, attributeValues bool) report.DriftReport {
	if attributeValues {
		return scanReport
	}
	return report.WithoutAttributeValues(scanReport)
}

func withHistoryLock(mu *sync.Mutex, fn func() error) error {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	return fn()
}

// finalizeRootScan runs the publish gate and side effects for one root.
// Policy runs first; on failure, history/artifacts/dashboard/notifications are skipped.
func finalizeRootScan(ctx context.Context, scanReport report.DriftReport, opts deliveryOptions) error {
	deliveryReport := persistableReport(scanReport, opts.AttributeValues)

	if opts.PolicyCommand != "" {
		if err := policy.Run(ctx, policy.Options{Command: opts.PolicyCommand, Args: opts.PolicyArgs}, deliveryReport); err != nil {
			return err
		}
	}

	if opts.ArtifactURL != "" {
		artifact, err := json.Marshal(deliveryReport)
		if err != nil {
			return fmt.Errorf("encode report artifact: %w", err)
		}
		if len(artifact) > maxArtifactBytes {
			return fmt.Errorf("report artifact exceeds %d bytes", maxArtifactBytes)
		}
		if err := (notify.ArtifactUploader{URL: opts.ArtifactURL}).Upload(ctx, artifact, "application/json"); err != nil {
			return err
		}
	}

	var historyEntries []history.Entry
	var previousReport report.DriftReport
	if opts.HistoryDir != "" {
		err := withHistoryLock(opts.historyMu, func() error {
			entries, err := history.LoadRecent(opts.HistoryDir, 100)
			if err != nil {
				return err
			}
			previousReport = previousReportForRoot(entries, scanReport)
			if shouldCreatePersistentIssue(scanReport, entries, opts.GitHubIssueAfter) {
				if err := (notify.GitHubIssueNotifier{Repository: opts.GitHubRepository, Token: os.Getenv("GITHUB_TOKEN")}).Notify(ctx, deliveryReport); err != nil {
					return err
				}
			}
			var historyWriteErr error
			if opts.HistoryCompressed {
				_, historyWriteErr = history.WriteCompressed(opts.HistoryDir, deliveryReport)
			} else {
				_, historyWriteErr = history.Write(opts.HistoryDir, deliveryReport)
			}
			if historyWriteErr != nil {
				return historyWriteErr
			}
			if opts.HistoryRetention > 0 {
				if err := history.Prune(opts.HistoryDir, opts.HistoryRetention); err != nil {
					return err
				}
			}
			entries, err = history.LoadRecent(opts.HistoryDir, 10)
			if err != nil {
				return err
			}
			historyEntries = entries
			return nil
		})
		if err != nil {
			return err
		}
	}

	if opts.DashboardHTMLPath != "" {
		if err := withHistoryLock(opts.historyMu, func() error {
			return writeDashboard(opts.DashboardHTMLPath, deliveryReport, historyEntries)
		}); err != nil {
			return err
		}
	}

	shouldNotify := !opts.NotificationThrottle || report.ShouldNotify(scanReport, previousReport)
	return deliverNotifications(ctx, opts.NotifyTarget, opts.SlackWebhookURL, opts.TeamsWebhookURL, opts.WebhookURL, opts.WebhookCACert, opts.GitHubRepository, opts.GitHubPR, opts.OwnerWebhooks, opts.NotificationThrottle, deliveryReport, previousReport, shouldNotify)
}
