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
- [x] Revisit Makefile formatting implementation if repository size grows significantly.

## Code quality plan

- [x] Move scan orchestration out of Cobra command code into `internal/scanner`.
- [x] Add typed scan outcomes so CLI exit codes can cleanly distinguish no drift, drift detected, and scan failure.
- [x] Decide whether `scan` should accept any directory or require at least one `.tf` file, then update docs/tests accordingly.
- [x] Add clear package boundaries for Terraform execution, plan parsing, reporting, and notification delivery.

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
- [ ] Bound notification response bodies and complete output/input size budgets.
- [ ] Redact `ErrorMessage` before JSON, history, dashboards, policy input, and notification delivery.
- [ ] Redact Terraform plan sensitive fields and marks before any future value expansion.
- [ ] Protect dashboard and history output paths from symlink writes.
- [ ] Document backend/state safety, secret scanning, and least-privilege AWS, Azure, and GCP IAM examples.
- [ ] Add release supply-chain controls: checksums, SBOM, provenance, SHA-pinned Actions examples, image signing, Docker scanning, and license policy checks.
- [ ] Add secret-safe structured audit logging with scan, config, workspace, version, and command metadata.
- [x] Test and reject unexpected Terraform detailed-exit codes; context cancellation is covered.
- [ ] Parse Terraform output-value changes, action reasons, replacement reasons, and deleted-resource context.
- [ ] Add safe Terraform initialization controls for lockfiles, upgrades, backends, and plugin caching.
- [ ] Add stable scan IDs to reports, history, notifications, and dashboards.
- [ ] Skip malformed history files with warnings instead of failing dashboard rendering.
- [x] Reject unknown config fields during loading.
- [ ] Validate config values early and publish a `.terradrift.json` JSON Schema.
- [ ] Add credential-free Terraform integration tests and golden plan-JSON fixtures.
- [x] Sort resource changes by address for deterministic output.
- [ ] Model complete, partial, and drift outcomes for multi-root scans.
- [ ] Stream or bound large Terraform JSON input and fail fast when Terraform roots have no configuration.
- [ ] Document Terraform provider/plugin caching with `TF_PLUGIN_CACHE_DIR` and CI cache keys.
- [ ] Parallelize independent post-scan enrichment and delivery work safely.
- [ ] Add incremental scan manifests, history retention pruning, and optional compressed history artifacts.
- [ ] Normalize workspace and output paths once through a validated execution context.
- [ ] Add typed option validation and typed errors across config, scan, notification, policy, and cost boundaries.
- [ ] Refresh stale package documentation and add complete comments for exported APIs.
- [ ] Add CLI help snapshots, a `make ci` target, coverage reports, changelog automation, and release automation.
- [ ] Add configuration examples, stable JSON schema guarantees, and package architecture/scan-sequence documentation.

## Ongoing quality gates

Before marking a feature complete, run the relevant checks and update this file:

- [x] `gofmt`
- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `go test -race ./...`
- [x] `golangci-lint run`
- [x] `govulncheck ./...`
