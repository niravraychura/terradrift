# TerraDrift Implementation Plan

This plan tracks security, performance, code quality, and feature work for TerraDrift. Checkboxes should only be marked complete when the implementation, tests, and documentation are done.

## Completed in this foundation pass

- [x] Add a repository-level `PLAN.md` so product and engineering work can be tracked in one place.
- [x] Ensure the global `--log-level` flag installs the configured `slog` logger instead of only validating the value.
- [x] Normalize `--output` values by trimming whitespace and accepting case-insensitive values.
- [x] Emit an empty `resource_changes` array in bootstrap JSON reports instead of `null`.
- [x] Add a reusable redaction package for high-risk strings before Terraform execution or notifications are implemented.
- [x] Add tests for output normalization, logger installation behavior, empty resource-change JSON, and redaction.

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

- [x] Install the configured logger as the process default logger.
- [x] Make output format parsing consistent with log-level parsing by accepting trimmed, case-insensitive values.
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
- [x] Add scheduled-run templates for GitHub Actions and cron/containerized runners with secret-safe defaults.
- [x] Add Microsoft Teams notification target with redacted connector-card payloads.
- [x] Add generic webhook notification target with SSRF-safe URL validation and payload controls.
- [x] Add historical report storage and dashboard trend views with restrictive local file permissions.
- [ ] Add policy-as-code hooks for expected drift and security guardrails.
- [ ] Add optional cost-impact enrichment for drifted resources.
- [ ] Add human-reviewed remediation guidance for update-code, sync-state, and revert-infrastructure workflows.

## Ongoing quality gates

Before marking a feature complete, run the relevant checks and update this file:

- [x] `gofmt`
- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `go test -race ./...`
- [x] `golangci-lint run`
- [ ] `govulncheck ./...`

`govulncheck` remains unchecked because the current environment returns HTTP 403 when fetching the Go vulnerability database. Mark it complete only after the vulnerability database can be reached and the command passes.
