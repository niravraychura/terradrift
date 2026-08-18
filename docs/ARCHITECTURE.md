# Architecture

TerraDrift is a CLI-first pipeline:

```text
cmd/terradrift -> config -> scanner -> terraform -> parser -> report
                                                -> enrich (cost/audit)
                                                -> stdout
                                                -> policy (publish gate)
                                                -> history/dashboard/artifacts/notifications
```

1. `cmd/terradrift` parses flags, loads config, and coordinates output.
2. `internal/scanner` validates roots, locks Terraform-backed scans, and assigns outcomes.
3. `internal/terraform` runs Terraform or OpenTofu with bounded command output.
4. `internal/parser` converts plan JSON into deterministic resource changes with safe attribute diffs.
5. `internal/report` adds risk, ownership, remediation, approvals, and audit metadata. `WithoutAttributeValues` clears Before/After for persistence defaults.
6. Policy runs before history, dashboard, artifact upload, and notifications. On failure those side effects are skipped.
7. `internal/history`, `dashboard`, and `notify` persist or deliver the publish-gated report.

## Attribute security contract

- **Always redacted**: Terraform sensitive marks and name heuristics (`password`, `secret`, `token`, `connection_string`, `db_url`, `user_data`, `private_key_pem`, `oauth`, and related markers).
- **Summarized**: large strings / encoded objects over 200 characters become `[changed, NB]`.
- **Stdout**: may include safe scalars and redacted/summarized values.
- **Default persistence/automation** (history, artifacts, policy stdin, dashboards, notifications): attribute **paths only** unless `--attribute-values` / `attribute_values` is set. Even then, secrets remain redacted.

External policy, cost, and audit adapters receive report JSON on standard input. They are explicit commands only; use configuration allowlists and trusted command directories in CI. Adapter stdout/stderr that exceeds the size budget fails closed.

# Report Stability

The JSON report fields `status`, `directory`, `total_resources_checked`, `total_changed_resources`, `resource_changes`, `started_at`, and `completed_at` are stable for automation. `root_id` is an optional stable opaque root identity, intended for history correlation when `directory` is path-redacted. New optional fields may be added. Consumers must ignore unknown fields and should not depend on text, dashboard, notification, SARIF, JUnit, or Prometheus wording as a stable API. Attribute `before`/`after` may be omitted (`omitempty`) in paths-only reports.
