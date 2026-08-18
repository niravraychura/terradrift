<!-- Archived 2026-07-22 repository review. Prefer ROADMAP.md and CHANGELOG.md for current direction. -->

# TerraDrift Repository Review and Improvement Backlog

Review date: 2026-07-22

## Project summary

TerraDrift is a self-hosted Terraform drift detection CLI written in Go. The repository currently provides a Cobra-based `terradrift` command with `scan` and `init` subcommands, bootstrap drift reports, optional Terraform CLI execution, JSON/table output, local history, static HTML dashboards, remediation guidance, policy/cost command hooks, Slack/Teams/generic webhook notifications, Docker packaging, CI, Dependabot, and a security policy.

The core scan path validates a local Terraform directory, can optionally constrain it to a workspace root, and can run `terraform init`, `terraform plan -refresh-only -detailed-exitcode -out <planfile>`, and `terraform show -json <planfile>`. Parsed resource changes are turned into a TerraDrift report with automation-friendly exit codes.

## Internet-informed context

The following current guidance and ecosystem patterns informed this backlog:

- HashiCorp documents refresh-only planning as a way to inspect remote-object changes made outside the normal Terraform workflow: <https://developer.hashicorp.com/terraform/cli/commands/plan>.
- HashiCorp's resource-drift tutorial recommends `terraform plan -refresh-only` to determine drift between the current state file and actual infrastructure: <https://developer.hashicorp.com/terraform/tutorials/state/resource-drift>.
- Terraform's JSON internals documentation describes machine-readable plan output suitable for automation: <https://developer.hashicorp.com/terraform/internals/json-format>.
- HashiCorp's Well-Architected Framework emphasizes continuous monitoring, automated validation, and clear remediation processes for configuration drift: <https://developer.hashicorp.com/well-architected-framework/optimize-systems/monitor-system-health/detect-configuration-drift>.
- HCP Terraform's drift detection feature highlights continuous checks and alerts as a common expectation for managed drift workflows: <https://www.hashicorp.com/en/lp/drift-detection-for-terraform-cloud>.
- OpenTofu issue discussions show user demand for structured drift data and remediation automation around `tofu plan -refresh-only -detailed-exitcode`: <https://github.com/opentofu/opentofu/issues/3315>.
- Industry drift guides from Terramate, Spacelift, env0, Scalr, and StackGen consistently emphasize scheduled scans, locked-down CI roles, remote state locking, policy-as-code, cost awareness, notifications, and human-reviewed remediation.

## High-priority security improvements

1. **Add an allowlist for external commands.** TerraDrift currently accepts arbitrary `--policy-command` and `--cost-command` values. Keep this flexibility for local use, but add config-level allowlists, explicit documentation, and CI-safe presets so shared runners do not execute unexpected binaries.
2. **Add command path validation.** Require external command names to be either bare executable names found on `PATH` or absolute paths under trusted directories. Reject shell metacharacter-like command strings even though `exec.CommandContext` does not invoke a shell by default.
3. **Add output-size and input-size budgets everywhere.** Terraform stderr/stdout and cost output are bounded, but policy output and notification response bodies should also have explicit byte limits to reduce log flooding and memory risk.
4. **Harden notification URL validation.** Require HTTPS, reject localhost/private-link destinations by default, and provide an override for intentionally private webhooks. This reduces SSRF-style misuse in multi-tenant or hosted runners.
5. **Redact report error fields before persistence and notification.** `ErrorMessage` can include provider diagnostics. Ensure redaction is applied before writing history, dashboards, JSON output, policy input, and all notifications.
6. **Add a sensitive-field redaction pass over Terraform plan JSON.** Even if resource changes currently store only address/type/name/actions, future parser expansion should traverse sensitive marks from Terraform JSON and never emit sensitive values.
7. **Protect generated dashboard files from symlink writes.** `os.OpenFile` follows symlinks. Add safe path handling or workspace-root enforcement for dashboard and history paths when running in automation.
8. **Add plan-file directory selection.** Temporary binary plan files are created inside the Terraform directory. Provide a `--work-dir` or secure temp directory option to avoid touching infrastructure repositories and to simplify cleanup policy.
9. **Add backend/state safety documentation.** Warn users that Terraform state and plans may contain secrets, recommend encrypted remote state, state locking, least-privilege read-only drift credentials where possible, and secret-manager-backed webhook credentials.
10. **Add supply-chain controls.** Publish release checksums/SBOMs/SLSA provenance, pin GitHub Actions by SHA in production examples, and add container image signing.
11. **Add Docker image vulnerability scanning.** Include Trivy or Grype in CI for the final runtime image.
12. **Add dependency license scanning.** Go dependencies are currently small, but release automation should fail on disallowed licenses.
13. **Add repository secret scanning guidance.** Document running GitHub secret scanning or gitleaks, especially because Terraform examples often include credentials by accident.
14. **Add least-privilege cloud IAM examples.** Provide AWS/Azure/GCP policy examples for drift scans that can refresh/read resources without applying mutations.
15. **Add audit logging mode.** Emit structured logs that include scan ID, config source, target workspace, Terraform version, and external command names without secrets.

## High-priority correctness and reliability improvements

1. **Clarify bootstrap vs Terraform execution documentation.** The README says the default emits a bootstrap no-drift report and also says real Terraform execution is planned next, while code already has explicit Terraform execution. Update wording so users understand what is implemented today.
2. **Capture Terraform version.** Include `terraform version -json` or equivalent metadata in reports to help debug provider/CLI drift behavior.
3. **Support OpenTofu.** Add `--terraform-bin` or `--runner-bin` so users can run `tofu` as well as `terraform`, and rename docs to avoid implying Terraform-only internals where OpenTofu is compatible.
4. **Handle Terraform detailed-exitcode defensively.** Preserve the current 0/1/2 behavior, but add tests for unexpected exit codes and context cancellation.
5. **Parse output changes.** Refresh-only plans can include output-value changes. Add `output_changes` support to reports and dashboards.
6. **Represent action reasons.** Terraform plan JSON can include useful metadata such as action reasons. Surface replacement reasons and deleted-resource context when available.
7. **Add provider/module initialization controls.** Support `-lockfile=readonly`, `-upgrade=false`, `-backend=false`, and plugin cache controls where appropriate for safe CI scans.
8. **Add scan IDs.** Include a stable UUID or timestamp-derived ID in reports, history filenames, notifications, and dashboard links.
9. **Make history loading resilient.** Skip malformed history files with warnings instead of failing the entire dashboard render.
10. **Add config validation.** Reject unsupported output formats, notify targets, negative timeouts, unsafe URLs, and unknown JSON fields during config loading instead of failing later.
11. **Add config schema.** Publish a JSON Schema for `.terradrift.json` to help editor validation and CI review.
12. **Add integration tests with real Terraform fixtures.** Keep unit tests credential-free, but add local-only Terraform fixtures using providers that do not require cloud credentials.
13. **Add golden JSON fixtures.** Validate parser behavior against representative Terraform plan JSON examples for create, update, delete, replace, read, no-op, import, and output changes.
14. **Add deterministic report ordering.** Sort resource changes by address so JSON, dashboards, notifications, and history diffs are stable.
15. **Add partial-failure semantics.** For multi-directory scans, distinguish complete failure, partial failure, and successful drift detection.

## Performance and scalability improvements

1. **Add multi-directory scanning.** Support scanning a manifest of Terraform roots with bounded concurrency, per-root timeouts, and aggregate reports.
2. **Stream large Terraform JSON parsing.** `terraform show -json` can be large. Consider streaming decoding or size limits with clear errors to protect memory.
3. **Add provider/plugin caching docs.** Document `TF_PLUGIN_CACHE_DIR` and CI cache keys to reduce scheduled-scan latency.
4. **Parallelize independent enrichment.** Cost, policy, dashboard generation, history writes, and notifications can be ordered carefully but some work can be concurrent once report mutation is complete.
5. **Add skip/no-op shortcuts.** If no `.tf` files or Terraform module markers exist, fail fast with a clearer message when `--terraform-exec` is enabled.
6. **Add incremental scan manifests.** Track root directories and last scan status in history so large mono-repos can focus on critical or changed environments.
7. **Bound dashboard history.** `LoadRecent` already limits loaded files after glob sorting; add retention pruning so large history directories do not grow forever.
8. **Compress historical reports.** Offer gzip compression for long-term history artifacts.
9. **Add benchmark tests.** Benchmark plan parsing, report rendering, redaction, and history loading with synthetic large reports.
10. **Avoid repeated path resolution.** Normalize workspace root and output paths once per command and pass a validated execution context through the pipeline.

## Code quality and maintainability improvements

1. **Split command orchestration.** `cmd/terradrift/main.go` has accumulated config merge, scan execution, enrichment, output, history, dashboard, policy, and notification logic. Move orchestration into an internal application service with smaller unit tests.
2. **Introduce typed options validation.** Add `Validate()` methods on config, scanner, notification, policy, and cost options to centralize guardrails.
3. **Use a report writer interface.** Separate table, JSON, dashboard, history, and notification formatting into focused packages.
4. **Add structured error types.** Use typed errors for invalid config, scan failure, drift detected, policy denied, notification failure, and external command failure.
5. **Add context-aware logging.** Thread a logger through scan operations instead of relying only on global slog state.
6. **Improve package docs.** Some package docs still say features “will” exist even though implementations now exist.
7. **Add comments for exported fields.** Go linting is currently minimal; exported types and methods would benefit from complete comments before enabling stricter lint rules.
8. **Add CLI help tests.** Snapshot help output for key flags so future changes do not accidentally hide important safety flags.
9. **Add `make ci`.** Combine format check, vet, tests, race tests, vulnerability scan, lint, and build into a single local command.
10. **Add test coverage reporting.** Produce `coverage.out` in CI and optionally upload coverage summaries.
11. **Add conventional commit and changelog automation.** This will help communicate security-sensitive behavior changes clearly.
12. **Add release automation.** Build multi-platform binaries and container images with signed checksums.
13. **Add examples for config files.** Provide `.terradrift.json` examples for local, GitHub Actions, cron, Slack, Teams, webhook, policy, cost, history, and dashboards.
14. **Document stability guarantees.** Define which JSON fields are stable before users automate against the report schema.
15. **Add architecture docs.** Include a package-level architecture diagram and scan sequence diagram.

## Feature backlog

1. **Multi-root workspace discovery.** Auto-detect Terraform roots in mono-repos and let users include/exclude paths.
2. **OpenTofu support.** First-class `tofu` binary selection, documentation, and tests.
3. **Policy packs.** Ship OPA/Rego and Conftest examples for high-risk drift, public exposure, missing tags, unencrypted storage, and production replacement gating.
4. **Cost-impact adapters.** Provide documented adapters for Infracost and custom cost APIs.
5. **Risk scoring.** Score drift by resource type, environment, action, tags, public exposure, and cost impact.
6. **Ownership mapping.** Map resource addresses or tags to service owners and route alerts accordingly.
7. **Notification throttling.** Avoid alert fatigue by suppressing repeated unchanged drift and escalating only when severity changes.
8. **GitHub issue creation.** Optionally open or update issues for persistent drift findings.
9. **SARIF output.** Emit SARIF so drift findings can appear in code-scanning dashboards.
10. **JUnit output.** Emit JUnit XML so CI systems can display drift by workspace as test failures.
11. **Prometheus metrics.** Export scan status, duration, resources checked, changed resources, and failure counts.
12. **Static web dashboard index.** Generate an index page across many roots and historical trends.
13. **Remediation runbooks.** Link resource types/actions to configurable runbook URLs.
14. **Human approval workflow.** Generate remediation suggestions that can be approved before any state/config changes.
15. **State reconciliation assistant.** Produce safe, review-only hints for `terraform import`, moved blocks, or configuration updates without applying changes automatically.
16. **Cloud event correlation.** Correlate drift with AWS CloudTrail, Azure Activity Log, or GCP Audit Logs to identify who changed what.
17. **Baseline/ignore rules.** Allow temporary exceptions with owners, reasons, expiration dates, and audit trails.
18. **Severity gates.** Configure which drift severities fail CI, notify only, or create tickets.
19. **Run summaries for pull requests.** Comment summarized scan results on PRs for infrastructure repositories.
20. **Remote artifact upload.** Upload reports and dashboards to S3, GCS, Azure Blob, or GitHub Pages with safe defaults.
21. **Config profiles.** Support named profiles for dev/stage/prod scan behavior.
22. **Workspace locks.** Avoid overlapping scans for the same Terraform root with local lock files or external lock integrations.
23. **Provider-aware enrichers.** Add AWS/Azure/GCP-specific context such as region, account/subscription/project, tags, encryption flags, and public exposure.
24. **Module/version inventory.** Report Terraform, provider, and module versions to support drift debugging and upgrade planning.
25. **Web UI API mode.** Add an optional read-only local API for serving reports from a history directory.

## Suggested near-term roadmap

1. **Security hardening release:** URL validation, external command validation/allowlists, redacted error propagation, safer output paths, and Docker scanning.
2. **Correctness release:** config validation, Terraform version metadata, output changes, parser golden tests, and documentation cleanup.
3. **Scale release:** multi-root scanning, bounded concurrency, retention pruning, and aggregate dashboards.
4. **Operations release:** risk scoring, alert throttling, ownership routing, SARIF/JUnit output, and Prometheus metrics.
5. **Ecosystem release:** OpenTofu support, Infracost adapter docs, OPA policy packs, and cloud audit-log correlation.
