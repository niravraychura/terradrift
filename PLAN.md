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
- [ ] Route all future Terraform stdout/stderr, notification payloads, and diagnostic logs through redaction before display.
- [ ] Add tests proving Slack webhook URLs, tokens, passwords, secrets, and authorization values are never printed in logs or errors.
- [ ] Add secure temporary plan-file handling for Terraform execution, including restrictive permissions and cleanup.
- [ ] Add optional path-redaction or relative-path output mode for CI logs.
- [ ] Evaluate symlink behavior and add optional workspace-root enforcement for hosted or multi-tenant runner scenarios.
- [ ] Document Terraform execution trust boundaries, including provider/module download risks and credential exposure risks.

## Performance plan

- [ ] Add scan-level context timeouts and a `--timeout` CLI flag before Terraform execution is introduced.
- [ ] Avoid buffering unbounded Terraform stdout/stderr in memory.
- [ ] Keep plan parsing focused on the fields needed for drift reports.
- [ ] Add large-plan parser fixtures or benchmarks once JSON parsing exists.
- [ ] Revisit Makefile formatting implementation if repository size grows significantly.

## Code quality plan

- [x] Install the configured logger as the process default logger.
- [x] Make output format parsing consistent with log-level parsing by accepting trimmed, case-insensitive values.
- [ ] Move scan orchestration out of Cobra command code into `internal/scanner`.
- [ ] Add typed scan outcomes so CLI exit codes can cleanly distinguish no drift, drift detected, and scan failure.
- [ ] Decide whether `scan` should accept any directory or require at least one `.tf` file, then update docs/tests accordingly.
- [ ] Add clear package boundaries for Terraform execution, plan parsing, reporting, and notification delivery.

## Feature plan

- [ ] Implement a Terraform CLI runner for `terraform init`, `terraform plan -refresh-only -detailed-exitcode`, and `terraform show -json`.
- [ ] Parse Terraform plan JSON into TerraDrift report models.
- [ ] Count checked resources and changed resources from real plan data.
- [ ] Wire exit code `2` for confirmed drift detection.
- [ ] Add configuration file support for repeated local and CI usage.
- [ ] Add a guided `terradrift init` command.
- [ ] Add Slack notifications only after redaction and secret-safe logging are fully tested.
- [ ] Add Docker runtime guidance for Terraform execution, including whether Terraform is bundled or mounted.
- [ ] Consider a self-hosted dashboard only after the CLI workflow is stable.

## Ongoing quality gates

Before marking a feature complete, run the relevant checks and update this file:

- [ ] `gofmt`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go test -race ./...`
- [ ] `golangci-lint run`
- [ ] `govulncheck ./...`
