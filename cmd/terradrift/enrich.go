package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/niravraychura/terradrift/internal/audit"
	"github.com/niravraychura/terradrift/internal/cost"
	"github.com/niravraychura/terradrift/internal/notify"
	"github.com/niravraychura/terradrift/internal/report"
)

func enrichReport(ctx context.Context, scanReport report.DriftReport, costCommand string, costArgs []string, auditCommand string, auditArgs []string) (report.DriftReport, error) {
	if costCommand == "" && auditCommand == "" {
		return scanReport, nil
	}
	if costCommand == "" {
		return audit.Enrich(ctx, audit.Options{Command: auditCommand, Args: auditArgs}, scanReport)
	}
	if auditCommand == "" {
		return cost.Enrich(ctx, cost.Options{Command: costCommand, Args: costArgs}, scanReport)
	}

	var costReport, auditReport report.DriftReport
	var costErr, auditErr error
	costInput := scanReport
	costInput.ResourceChanges = append([]report.ResourceChange(nil), scanReport.ResourceChanges...)
	auditInput := scanReport
	auditInput.ResourceChanges = append([]report.ResourceChange(nil), scanReport.ResourceChanges...)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		costReport, costErr = cost.Enrich(ctx, cost.Options{Command: costCommand, Args: costArgs}, costInput)
	}()
	go func() {
		defer workers.Done()
		auditReport, auditErr = audit.Enrich(ctx, audit.Options{Command: auditCommand, Args: auditArgs}, auditInput)
	}()
	workers.Wait()
	if err := errors.Join(costErr, auditErr); err != nil {
		return scanReport, err
	}
	costByAddress := make(map[string]report.ResourceChange, len(costReport.ResourceChanges))
	for _, change := range costReport.ResourceChanges {
		costByAddress[change.Address] = change
	}
	auditByAddress := make(map[string]report.ResourceChange, len(auditReport.ResourceChanges))
	for _, change := range auditReport.ResourceChanges {
		auditByAddress[change.Address] = change
	}
	for i, change := range scanReport.ResourceChanges {
		if costChange, ok := costByAddress[change.Address]; ok {
			scanReport.ResourceChanges[i].CostImpact = costChange.CostImpact
		}
		if auditChange, ok := auditByAddress[change.Address]; ok {
			scanReport.ResourceChanges[i].AuditEvents = auditChange.AuditEvents
		}
	}
	return scanReport, nil
}

type deliveryTask struct {
	name string
	run  func() error
}

func deliverNotifications(ctx context.Context, target, slackWebhookURL, teamsWebhookURL, webhookURL, webhookCACert, githubRepository string, githubPR int, ownerWebhooks map[string]string, throttle bool, scanReport, previousReport report.DriftReport, shouldNotify bool) error {
	if !shouldNotify {
		return nil
	}
	tasks := make([]deliveryTask, 0, len(ownerWebhooks)+2)
	if target != "" {
		tasks = append(tasks, deliveryTask{name: "notification", run: func() error {
			return sendNotification(ctx, target, slackWebhookURL, teamsWebhookURL, webhookURL, webhookCACert, scanReport)
		}})
	}
	if githubRepository != "" && githubPR > 0 {
		tasks = append(tasks, deliveryTask{name: "github pull request", run: func() error {
			return (notify.GitHubPRNotifier{Repository: githubRepository, Number: githubPR, Token: os.Getenv("GITHUB_TOKEN")}).Notify(ctx, scanReport)
		}})
	}
	owners := make([]string, 0, len(ownerWebhooks))
	for owner := range ownerWebhooks {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		ownerReport := reportForOwner(scanReport, owner)
		if len(ownerReport.ResourceChanges) == 0 || (throttle && !report.ShouldNotify(ownerReport, reportForOwner(previousReport, owner))) {
			continue
		}
		webhook := ownerWebhooks[owner]
		tasks = append(tasks, deliveryTask{name: "owner " + owner, run: func() error {
			return (notify.WebhookNotifier{WebhookURL: webhook, CACertPath: webhookCACert}).Notify(ctx, ownerReport)
		}})
	}
	return runDeliveries(tasks)
}

func reportForOwner(scanReport report.DriftReport, owner string) report.DriftReport {
	ownerReport := scanReport
	ownerReport.ResourceChanges = nil
	for _, change := range scanReport.ResourceChanges {
		if change.Owner == owner && !change.Ignored {
			ownerReport.ResourceChanges = append(ownerReport.ResourceChanges, change)
		}
	}
	ownerReport.TotalChangedResources = len(ownerReport.ResourceChanges)
	return ownerReport
}

func runDeliveries(tasks []deliveryTask) error {
	if len(tasks) == 0 {
		return nil
	}
	jobs := make(chan deliveryTask)
	errs := make(chan error, len(tasks))
	var workers sync.WaitGroup
	for range min(maxDeliveryWorkers, len(tasks)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range jobs {
				if err := task.run(); err != nil {
					errs <- fmt.Errorf("%s delivery: %w", task.name, err)
				}
			}
		}()
	}
	for _, task := range tasks {
		jobs <- task
	}
	close(jobs)
	workers.Wait()
	close(errs)
	var all []error
	for err := range errs {
		all = append(all, err)
	}
	return errors.Join(all...)
}

func sendNotification(ctx context.Context, target string, slackWebhookURL string, teamsWebhookURL string, webhookURL string, webhookCACert string, scanReport report.DriftReport) error {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "slack":
		return notify.SlackNotifier{WebhookURL: slackWebhookURL, CACertPath: webhookCACert}.Notify(ctx, scanReport)
	case "teams":
		return notify.TeamsNotifier{WebhookURL: teamsWebhookURL, CACertPath: webhookCACert}.Notify(ctx, scanReport)
	case "webhook":
		return notify.WebhookNotifier{WebhookURL: webhookURL, CACertPath: webhookCACert}.Notify(ctx, scanReport)
	default:
		return fmt.Errorf("unsupported notification target %q; supported values: slack, teams, webhook", target)
	}
}
