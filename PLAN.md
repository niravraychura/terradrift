# TerraDrift Implementation Plan

This plan tracks security, performance, code quality, and feature work for TerraDrift. Checkboxes should only be marked complete when the implementation, tests, and documentation are done.

## Completed in this foundation pass

- [x] Add a repository-level `PLAN.md` so product and engineering work can be tracked in one place.
- [x] Normalize `--output` values by trimming whitespace and accepting case-insensitive values.
- [x] Emit an empty `resource_changes` array in bootstrap JSON reports instead of `null`.
- [x] Add a reusable redaction package for high-risk strings before Terraform execution or notifications are implemented.
- [x] Add tests for output normalization, empty resource-change JSON, and redaction.

## Security plan

- [x] Add central redaction helpers for webhook URLs, common credential assignment patterns, and sensitive query parameters.
- [x] Route all future Terraform stdout/stderr, notification payloads, and diagnostic logs through redaction before display.
- [x] Add tests proving Slack webhook URLs, tokens, passwords, secrets, and authorization values are never printed in logs or errors.
- [x] Add secure temporary plan-file handling for Terraform execution, including restrictive permissions and cleanup.
- [x] Add optional path-redaction or relative-path output mode for CI logs.
- [x] Evaluate symlink behavior and add optional workspace-root enforcement for hosted or multi-tenant runner scenarios.
- [x] Document Terraform execution trust boundaries, including provider/module download risks and credential exposure risks.

## Performance plan

- [x] Add scan-level context timeouts and a `--timeout` CLI flag before Terraform execution is introduced.
- [x] Avoid buffering unbounded Terraform stdout/stderr in memory.
- [x] Keep plan parsing focused on the fields needed for drift reports.
- [x] Add large-plan parser fixtures or benchmarks once JSON parsing exists.
- [x] Add benchmark tests for report rendering, redaction, and history loading with synthetic large reports.
- [x] Revisit Makefile formatting implementation if repository size grows significantly.

## Code quality plan

- [x] Move scan orchestration out of Cobra command code into `internal/scanner`.
- [x] Add typed scan outcomes so CLI exit codes can cleanly distinguish no drift, drift detected, and scan failure.
- [x] Decide whether `scan` should accept any directory or require at least one `.tf` file, then update docs/tests accordingly.
- [x] Add clear package boundaries for Terraform execution, plan parsing, reporting, and notification delivery.
- [x] Thread a context-aware logger through scan operations instead of relying only on global slog state.

## Feature plan

- [x] Implement a Terraform CLI runner for `terraform init`, `terraform plan -refresh-only -detailed-exitcode`, and `terraform show -json`.
- [x] Parse Terraform plan JSON into TerraDrift report models.
- [x] Count checked resources and changed resources from real plan data.
- [x] Wire exit code `2` for confirmed drift detection.
- [x] Add configuration file support for repeated local and CI usage, including advanced scan options that were previously CLI-only.
- [x] Add a guided `terradrift init` command.
- [x] Add Slack notifications only after redaction and secret-safe logging are fully tested.
- [x] Add Docker runtime guidance for Terraform execution, including whether Terraform is bundled or mounted.
- [x] Consider a self-hosted dashboard only after the CLI workflow is stable.
- [x] Add scheduled-run templates for GitHub Actions and cron runners with secret-safe defaults.
- [x] Add Microsoft Teams notification target with redacted connector-card payloads.
- [x] Harden generic webhook delivery with DNS- and redirect-safe SSRF validation.
- [x] Add historical report storage and dashboard trend views with restrictive local file permissions.
- [x] Add policy-as-code hooks for expected drift and security guardrails with no implicit shell execution and redacted bounded output.
- [x] Add optional cost-impact enrichment for drifted resources with explicit external command arguments and redacted bounded output.
- [x] Add human-reviewed remediation guidance for update-code, sync-state, and revert-infrastructure workflows.

## Future feature backlog

- [x] Add manifest-based multi-root scans with bounded concurrency and aggregate reports.
- [x] Add automatic multi-root workspace discovery with include and exclude paths.
- [x] Add first-class OpenTofu binary selection, documentation, and tests.
- [x] Ship OPA and Conftest policy-pack examples for high-risk drift.
- [x] Document adapters for Infracost and custom cost APIs.
- [x] Add conservative action-based risk scoring.
- [x] Add exact-address and resource-type ownership mapping with HTTPS webhook routing.
- [x] Add notification throttling for unchanged drift with severity-based escalation.
- [x] Add optional GitHub issue creation for persistent drift findings.
- [x] Add SARIF output for code-scanning dashboards.
- [x] Add JUnit XML output for CI test reporting.
- [x] Add Prometheus metrics for scan status, duration, resource counts, and failures.
- [x] Add a static dashboard index across roots and historical trends.
- [x] Add configurable remediation runbook URLs by resource type and action.
- [x] Add a human approval workflow for remediation suggestions.
- [x] Add review-only state reconciliation hints for imports, moved blocks, and configuration updates.
- [x] Add cloud audit-event correlation through explicit external adapters.
- [x] Add baseline and ignore rules with owners, reasons, expirations, and audit trails.
- [x] Add severity gates for CI failure behavior.
- [x] Add GitHub pull-request scan-result summaries for infrastructure repositories.
- [x] Add remote report artifact upload through presigned HTTPS URLs.
- [x] Add named configuration profiles for development, staging, and production scans.
- [x] Add workspace locks to prevent overlapping scans of the same Terraform root.
- [x] Add provider-aware AWS, Azure, and GCP classification from Terraform metadata.
- [x] Add Terraform, provider, and module version inventory reporting.
- [x] Add an optional read-only local API for serving report history.

## Review Follow-ups

- [x] Add external-command allowlists and trusted-path validation; use profiles for CI-safe presets.
- [x] Bound notification response bodies.
- [x] Bound config and history report reads and history writes.
- [x] Bound report input sent to external policy, cost, and audit adapters.
- [x] Bound uploaded JSON report artifacts.
- [x] Bound report and approval artifact file inputs.
- [x] Complete report output/input size budgets with bounded storage, adapter, approval, notification, and artifact boundaries; stream user-facing output.
- [x] Redact `ErrorMessage` before JSON, history, dashboards, policy input, and notification delivery.
- [x] Exclude Terraform plan values and sensitive marks from report parsing, with regression coverage.
- [x] Reject direct symlink targets for dashboard and history output paths.
- [x] Document backend/state safety and secret-managed integration credentials.
- [x] Document repository secret scanning and least-privilege AWS, Azure, and GCP IAM guidance.
- [x] Pin third-party GitHub Actions to immutable SHAs in CI and user-facing examples.
- [x] Automate tag releases with SHA-256 checksums and generated GitHub release notes.
- [x] Add SBOM, provenance, image signing/scanning, and license policy checks.
- [x] Add secret-safe structured audit logging with scan, config, workspace, version, and command metadata.
- [x] Test and reject unexpected Terraform detailed-exit codes; context cancellation is covered.
- [x] Parse Terraform output-change actions and resource action reasons without retaining Terraform values or sensitive marks.
- [x] Use read-only Terraform lockfiles, disable implicit upgrades, retain backend initialization, and document isolated plugin caching.
- [x] Add stable scan IDs to reports, history, notifications, and dashboards.
- [x] Skip malformed history files with warnings instead of failing dashboard rendering.
- [x] Reject unknown config fields during loading.
- [x] Validate core config values early during loading.
- [x] Publish a `.terradrift.json` JSON Schema.
- [x] Add credential-free Terraform integration tests and golden plan-JSON fixtures.
- [x] Sort resource changes by address for deterministic output.
- [x] Model complete, partial, failed, and drift outcomes for multi-root scans.
- [x] Bound Terraform command output and fail explicitly on truncation.
- [x] Fail fast when Terraform-backed roots have no configuration.
- [x] Document Terraform provider/plugin caching with `TF_PLUGIN_CACHE_DIR` and CI cache keys.
- [x] Parallelize independent post-scan enrichment and delivery work safely.
- [x] Add local history retention pruning.
- [x] Add incremental scan manifests and optional compressed history artifacts.
- [x] Normalize workspace and output paths once through a validated execution context.
- [x] Add typed option validation and typed errors across config, scan, notification, policy, and cost boundaries.
- [x] Refresh stale package documentation for implemented behavior.
- [x] Add complete comments for exported package APIs.
- [x] Add a non-mutating `make ci` target for local quality checks.
- [x] Add CLI help safety-flag coverage.
- [x] Produce CI coverage output and function summaries.
- [x] Add generated changelog notes and tag-driven release automation.
- [x] Add supported local and CI configuration examples.
- [x] Document stable JSON report guarantees and package architecture/scan-sequence design.

## Ongoing quality gates

Before marking a feature complete, run the relevant checks and update this file:

- [x] `gofmt`
- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `go test -race ./...`
- [x] `golangci-lint run`
- [x] `govulncheck ./...`

## MonkeyCode review follow-ups

Verified against the current codebase. Keep completed history above; track only actionable follow-ups here. Overstated or outdated review claims are omitted (parser edge-case coverage already exists; report/config/notify tests already exist; cost/audit enrichment already merges by address).

### High

- [x] Split oversized CLI entry point: move `newScanCommand` / `newScanAllCommand` handlers and helpers (`writeScanReport`, `enrichReport`, `deliverNotifications`, `runDeliveries`) out of `cmd/terradrift/main.go` into focused files or internal packages.

### Medium

- [x] Extract shared `limitedBuffer` from `internal/cost`, `internal/policy`, and `internal/audit` into one shared package.
- [x] Extract shared `readLimitedFile` from `cmd/terradrift` and `internal/terraform` into one shared package.
- [x] Replace repetitive config-to-CLI `Flags().Changed(...)` mapping with a table-driven overlay helper.
- [x] Make `enrichReport` merge cost/audit fields by resource `Address` (defense-in-depth; Enrich already attaches by address today).
- [x] Narrow sensitive query-parameter redaction so matching `"key"` as a substring does not false-positive on benign names like `key_id`.
- [x] Add unit tests for `internal/validation`.
- [x] Add an optional external/distributed scan-lock backend for multi-runner CI (local `O_EXCL` lock remains for single-host use).

### Low

- [x] Replace `panic` on `MarkFlagRequired` failures with controlled startup error handling.
- [x] Validate `GITHUB_TOKEN` early when GitHub notification delivery is configured.
- [x] Document or harden workspace-root symlink TOCTOU between validation and Terraform execution.
- [x] Add optional webhook custom CA support for enterprise TLS environments.
- [x] Optimize `history.LoadRecent` to avoid sorting the full file list when only a small limit is needed.
- [x] Investigate optional skip/reuse of `terraform init` when providers and lockfile are already satisfied.
- [x] Add remaining quality benchmarks for report rendering, redaction, and history loading with synthetic large reports.
- [x] Thread a context-aware logger through scan operations instead of relying only on global slog state.

## Next hardening backlog

Agreed follow-ups from the post-MonkeyCode security/resilience/performance review. Implement in priority order. Checkboxes move to `[x]` only when code, tests, and docs are done.

### 1. Attribute diffs with security (product + security)

Keep attribute-level change reporting. Do not remove diffs. Change what values are shown and where they are stored.

**Display rules (table / human output and JSON `attribute_changes`):**

- Always include the attribute **path** when a resource changed.
- Show **old → new values** only when safe:
  - Terraform `before_sensitive` / `after_sensitive` → `"[REDACTED]" -> "[REDACTED]"` (or redact the sensitive side only).
  - Sensitive name heuristics (`password`, `secret`, `token`, `*_key`, `api_key`, `access_key`, `private_key`, `aws_access_key_id`, `secret_string`, credentials, etc.) → redacted even if unmarked.
  - Safe scalars (numbers, bools, short non-secret strings, enums) → real values (e.g. `idle_timeout: 60 -> 120`).
  - Large / blob values (IAM policy JSON, `user_data`, container definitions) → summarize or truncate (e.g. `policy: [changed, 4KB]`), never dump full blobs by default.
  - Unknown after apply → `(known after apply)`.
- `(absent)` remains valid for create/delete sides.

**Persistence / automation rules:**

- Default for **history, dashboard artifacts, uploaded report artifacts, policy stdin, and notifications**: either the same redacted view as above, or **paths-only** (path + actions, no raw values).
- Add an explicit opt-in to include safe values in persisted/automation output (e.g. `--attribute-values` and/or config `attribute_values`). Sensitive values must still never appear in cleartext.
- Document the contract in README and `docs/ARCHITECTURE.md` (what is always redacted vs optional).

**Implementation notes:**

- Primary code: `internal/parser/diff.go`, `internal/report` (`AttributeChange`), `cmd/terradrift/output.go`, history/notify/policy wiring in `scan.go`.
- Expand heuristics and tests (including unmarked secret-like names such as `connection_string` / `db_url` if adopted).
- Regression tests must prove secret fixtures never appear in history JSON, policy input, or notification payloads.

- [x] Implement safe attribute value display rules (sensitive marks + heuristics + truncation/summaries).
- [x] Default history / artifacts / policy / notifications to redacted or paths-only attribute data.
- [x] Add opt-in `--attribute-values` (and config) for including safe values in persisted/automation output; secrets remain redacted.
- [x] Document attribute security contract in README and architecture docs; add regression tests.

### 2. Policy as a publish gate (resilience + security)

- [x] Run `policy.Run` **before** writing history, dashboard HTML, uploaded artifacts, and sending notifications.
- [x] On policy failure, do not persist or upload a success-looking report; return a clear scan failure.
- [x] Update tests and README for the new pipeline order.

### 3. Harden GitHub HTTP delivery (security)

- [x] Route GitHub issue/PR notification HTTP through the same SSRF-safe dialer / no-redirect / no-proxy posture used for webhooks (`internal/notify`).
- [x] Add explicit client timeouts (connect / TLS / overall), independent of the long scan context where practical.
- [x] Add tests covering blocked hosts / redirects / token-safe errors.

### 4. Fail closed on truncated adapter output (resilience)

- [x] Make policy / cost / audit command capture fail when stdout/stderr exceeds the size budget (same fail-on-truncate behavior as Terraform `limitedWriter`), instead of silently truncating then parsing.
- [x] Prefer a shared truncated-aware buffer in `internal/ioutil`; update callers and tests.

### 5. Large plan performance (performance)

- [x] Stream or selectively decode `terraform show -json` so full `prior_state` / unused fields are not held when only counts and changed resources are needed.
- [x] Keep attribute-diff extraction bounded; avoid walking huge unchanged blobs.
- [x] Add a large-plan fixture or benchmark gate for parse memory/CPU regressions.

### 6. Lock honesty and docs (correctness)

Note: `--lock-backend` currently supports **`local` only** (per-host `O_EXCL` file lock). PLAN previously implied a distributed backend; that is not implemented.

- [x] Document clearly in README and SECURITY.md that the scan lock is local/single-host unless runners share the lock file via a shared filesystem.
- [x] Optionally: add stale-lock guidance (PID check / manual removal) and keep Redis/Postgres backends explicitly out of scope until productized.

### Suggested implementation order

1. Attribute security (§1)
2. Policy publish gate (§2)
3. GitHub client hardening (§3)
4. Adapter truncate-fail (§4)
5. Stream/selective plan decode (§5)
6. Lock docs honesty (§6)

## Next product backlog

Product and UX gaps beyond hardening. Do **not** start these until the Next hardening backlog is mostly complete, except where noted. Prefer one product Must per sprint.

### Must (after hardening)

- [x] Bring `scan-all` to delivery parity with `scan`: support history, dashboard, notifications, policy (as publish gate), and cost/audit enrichment for multi-root runs (or a clear documented subset that is production-usable). Today multi-root intentionally skips most post-scan delivery.
- [x] Add Terraform workspace selection and `-var` / `-var-file` passthrough (CLI + config) so one root can target multiple workspaces/env files without requiring separate directories.

### Should

- [x] Stop treating bootstrap no-drift as the default meaningful result: either require `--terraform-exec` for real scans, or keep bootstrap only behind an explicit dry-run/bootstrap flag and fail/warn loudly otherwise.
- [x] Make Slack/Teams/webhook notifications actionable: include top-N resource addresses, risk levels, and a pointer to the report (still redacted; no secret attribute values).
- [x] Support per-root settings in manifests (at least profile / plan_mode / var-files), so mono-repos are not forced into one global flag set.

### UX / DX (cheap wins)

- [x] Rewrite README current-status section to reflect what is production-usable today; demote “foundation only” language that undersells shipped features.
- [x] Emit scan phase progress lines (`init`, `plan`, `show`, `parse`) so long Terraform runs do not look hung (compatible with `--redact-paths`).
- [x] Fix stale docs/examples that claim reports lack attributes (e.g. policy examples) so they match `attribute_changes`.
- [x] Ship one end-to-end example: scheduled multi-root + Slack + severity gate (config + GitHub Actions).
- [x] Clarify exit codes vs `--failure-severity` with one concrete CI example near the flags/docs.
- [x] Improve `scan-all` aggregate table/JSON so partial failure vs drift per root is obvious without reading every nested report.
- [x] Group advanced `scan` flags in help/docs (core / delivery / enrichment) to reduce flag overload.

### Explicitly out of scope for now

- Hosted SaaS / multi-tenant auth for `serve`
- Redis/Postgres distributed lock backends
- Auto-apply / auto-import / any state mutation
- Additional chat adapters before notifications are useful
- Embedded Infracost/OPA (keep external adapters)
- Fancy interactive TUI or large React dashboard rewrite

### Product implementation order (after hardening)

1. `scan-all` delivery parity
2. Workspace + var-file support
3. Bootstrap-default honesty
4. Actionable notifications
5. Per-root manifest settings
6. UX/DX doc and progress polish
