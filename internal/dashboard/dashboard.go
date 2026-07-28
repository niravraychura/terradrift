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

var indexTemplate = template.Must(template.New("dashboard-index").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>TerraDrift Dashboard Index</title></head>
<body>
  <main>
    <h1>TerraDrift Dashboard Index</h1>
    <table>
      <thead><tr><th>Scan ID</th><th>Directory</th><th>Completed at</th><th>Status</th><th>Plan mode</th><th>Resources checked</th><th>Changed resources</th></tr></thead>
      <tbody>{{range .}}<tr><td>{{.Report.ScanID}}</td><td>{{.Report.Directory}}</td><td>{{.Report.CompletedAt}}</td><td>{{.Report.Status}}</td><td>{{.Report.PlanMode}}</td><td>{{.Report.TotalResourcesChecked}}</td><td>{{.Report.TotalChangedResources}}</td></tr>{{else}}<tr><td colspan="7">No history available</td></tr>{{end}}</tbody>
    </table>
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

// RenderIndex writes an escaped static index across recent scan history.
func RenderIndex(w io.Writer, entries []history.Entry) error {
	if err := indexTemplate.Execute(w, entries); err != nil {
		return fmt.Errorf("render dashboard index: %w", err)
	}
	return nil
}
