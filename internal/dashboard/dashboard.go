// Package dashboard renders local static HTML scan reports.
package dashboard

import (
	"fmt"
	"html/template"
	"io"

	"github.com/niravraychura/terradrift/internal/history"
	"github.com/niravraychura/terradrift/internal/report"
)

// Data contains the current scan and optional historical scan reports.
type Data struct {
	Current report.DriftReport
	History []history.Entry
	Trend   Trend
}

// Trend summarizes recent scan outcomes without retaining additional report data.
type Trend struct {
	Scans   int
	Drifted int
	Changes int
	Failed  int
}

var reportTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>TerraDrift Report</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 1.5rem; color: #1a1a1a; line-height: 1.4; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #d0d0d0; padding: 0.4rem 0.6rem; text-align: left; }
    th { background: #f3f3f3; }
    h2 { margin-top: 1.5rem; }
  </style>
</head>
<body>
  <main>
    <h1>TerraDrift Report</h1>
    <dl>
       <dt>Scan ID</dt><dd>{{.Current.ScanID}}</dd>
       <dt>Status</dt><dd>{{.Current.Status}}</dd>
       <dt>Plan mode</dt><dd>{{.Current.PlanMode}}</dd>
       <dt>Resources checked</dt><dd>{{.Current.TotalResourcesChecked}}</dd>
      <dt>Changed resources</dt><dd>{{.Current.TotalChangedResources}}</dd>
    </dl>
    <h2>Changed resources</h2>
    <table>
      <thead><tr><th>Address</th><th>Cloud</th><th>Type</th><th>Name</th><th>Actions</th><th>Cost impact</th><th>Remediation</th><th>Reconciliation</th><th>Ignore</th><th>Runbook</th></tr></thead>
      <tbody>{{range .Current.ResourceChanges}}<tr><td>{{.Address}}</td><td>{{.CloudProvider}}</td><td>{{.Type}}</td><td>{{.Name}}</td><td>{{range .Actions}}{{.}} {{end}}</td><td>{{.CostImpact}}</td><td>{{.Remediation}}</td><td>{{.ReconciliationHint}}</td><td>{{if .Ignored}}{{.IgnoreOwner}}: {{.IgnoreReason}} (until {{.IgnoreExpiresAt}}){{end}}</td><td>{{if .RunbookURL}}<a href="{{.RunbookURL}}">Open</a>{{end}}</td></tr>{{else}}<tr><td colspan="10">No changed resources</td></tr>{{end}}</tbody>
    </table>
     <h2>Recent scan history</h2>
	    <p>Trend: {{.Trend.Drifted}} drifted, {{.Trend.Changes}} with configuration changes, and {{.Trend.Failed}} failed scans across {{.Trend.Scans}} recent scans.</p>
    <table>
      <thead><tr><th>Scan ID</th><th>Completed at</th><th>Status</th><th>Plan mode</th><th>Resources checked</th><th>Changed resources</th></tr></thead>
      <tbody>{{range .History}}<tr><td>{{.Report.ScanID}}</td><td>{{.Report.CompletedAt}}</td><td>{{.Report.Status}}</td><td>{{.Report.PlanMode}}</td><td>{{.Report.TotalResourcesChecked}}</td><td>{{.Report.TotalChangedResources}}</td></tr>{{else}}<tr><td colspan="6">No history available</td></tr>{{end}}</tbody>
    </table>
  </main>
</body>
</html>
`))

type indexGroup struct {
	Directory string
	Entries   []history.Entry
}

type indexData struct {
	Groups []indexGroup
}

var indexTemplate = template.Must(template.New("dashboard-index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>TerraDrift Dashboard Index</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 1.5rem; color: #1a1a1a; line-height: 1.4; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #d0d0d0; padding: 0.4rem 0.6rem; text-align: left; }
    th { background: #f3f3f3; }
    h2 { margin-top: 1.5rem; font-size: 1.1rem; }
  </style>
</head>
<body>
  <main>
    <h1>TerraDrift Dashboard Index</h1>
    {{range .Groups}}
    <h2>{{.Directory}}</h2>
    <table>
      <thead><tr><th>Scan ID</th><th>Completed at</th><th>Status</th><th>Plan mode</th><th>Resources checked</th><th>Changed resources</th></tr></thead>
      <tbody>{{range .Entries}}<tr><td>{{.Report.ScanID}}</td><td>{{.Report.CompletedAt}}</td><td>{{.Report.Status}}</td><td>{{.Report.PlanMode}}</td><td>{{.Report.TotalResourcesChecked}}</td><td>{{.Report.TotalChangedResources}}</td></tr>{{end}}</tbody>
    </table>
    {{else}}<p>No history available</p>{{end}}
  </main>
</body>
</html>
`))

// RenderWithHistory writes a static, escaped HTML dashboard with optional history.
func RenderWithHistory(w io.Writer, data Data) error {
	data.Trend = trendFor(data.History)
	if err := reportTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("render dashboard: %w", err)
	}
	return nil
}

func trendFor(entries []history.Entry) Trend {
	trend := Trend{Scans: len(entries)}
	for _, entry := range entries {
		switch entry.Report.Status {
		case report.ScanStatusDriftDetected:
			trend.Drifted++
		case report.ScanStatusChangesDetected:
			trend.Changes++
		case report.ScanStatusFailed:
			trend.Failed++
		}
	}
	return trend
}

// RenderIndex writes an escaped static index grouped by Terraform root directory.
func RenderIndex(w io.Writer, entries []history.Entry) error {
	if err := indexTemplate.Execute(w, indexData{Groups: groupIndexEntries(entries)}); err != nil {
		return fmt.Errorf("render dashboard index: %w", err)
	}
	return nil
}

func groupIndexEntries(entries []history.Entry) []indexGroup {
	order := make([]string, 0)
	byDir := make(map[string][]history.Entry)
	for _, entry := range entries {
		dir := entry.Report.Directory
		if _, ok := byDir[dir]; !ok {
			order = append(order, dir)
		}
		byDir[dir] = append(byDir[dir], entry)
	}
	groups := make([]indexGroup, 0, len(order))
	for _, dir := range order {
		groups = append(groups, indexGroup{Directory: dir, Entries: byDir[dir]})
	}
	return groups
}
