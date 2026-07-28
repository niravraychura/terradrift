# Architecture

TerraDrift is a CLI-first pipeline:

```text
cmd/terradrift -> config -> scanner -> terraform -> parser -> report
                                                -> policy/cost/audit adapters
                                                -> history/dashboard/notifications
```

1. `cmd/terradrift` parses flags, loads config, and coordinates output.
2. `internal/scanner` validates roots, locks Terraform-backed scans, and assigns outcomes.
3. `internal/terraform` runs Terraform or OpenTofu with bounded command output.
4. `internal/parser` converts plan JSON into deterministic resource changes.
5. `internal/report` adds risk, ownership, remediation, approvals, and audit metadata.
6. `internal/history`, `dashboard`, and `notify` persist or deliver already-built reports.

External policy, cost, and audit adapters receive report JSON on standard input. They are explicit commands only; use configuration allowlists and trusted command directories in CI.

# Report Stability

The JSON report fields `status`, `directory`, `total_resources_checked`, `total_changed_resources`, `resource_changes`, `started_at`, and `completed_at` are stable for automation. New optional fields may be added. Consumers must ignore unknown fields and should not depend on text, dashboard, notification, SARIF, JUnit, or Prometheus wording as a stable API.
