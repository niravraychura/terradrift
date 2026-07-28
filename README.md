# TerraDrift

TerraDrift is an open-source, self-hosted Terraform drift detection tool. It is being built to help teams identify infrastructure changes that happened outside their normal Terraform workflow and surface those changes in a clear, automation-friendly way.

> [!WARNING]
> TerraDrift is under active development. The default scan still emits a bootstrap report unless `--terraform-exec` is explicitly enabled. Treat Terraform execution, Slack notifications, and static dashboard output as early CLI features and review their output before relying on them in production automation.

## Why TerraDrift exists

Terraform is the source of truth for many infrastructure teams, but real environments can still drift when resources are changed manually, by emergency scripts, by provider-side defaults, or by systems outside Terraform. TerraDrift aims to provide a simple self-hosted workflow for detecting that drift without requiring a hosted SaaS platform.

The long-term goal is to make drift detection easy to run from a developer laptop, CI pipeline, scheduled job, or private automation runner, then publish clear results to humans and automation systems.

## Current status

The first version of TerraDrift is a project foundation. It includes:

- A Go module and Cobra-based CLI named `terradrift`
- A `scan` command that defaults to the current directory and supports `--directory` / `-d`
- Directory validation, optional workspace-root enforcement, and absolute path reporting
- Human-friendly table output and automation-friendly JSON output
- Documented exit codes for future CI drift workflows
- Domain models for future drift reports
- A Terraform runner interface and explicit Terraform CLI execution mode
- Secret-safe Slack notifications and static HTML dashboard output
- Unit tests that do not require Terraform or cloud credentials
- Docker, Makefile, GitHub Actions CI, Dependabot, and security policy scaffolding

Terraform execution is available behind the explicit `--terraform-exec` flag while the broader workflow continues to mature.

## Do Terraform files need to be in this repository?

No. Terraform code does **not** need to live in the TerraDrift repository.

TerraDrift is intended to scan any local Terraform directory that is available to the process at runtime. That directory can come from:

- The same repository as TerraDrift during local development
- A separate infrastructure repository checked out by CI
- A mounted directory in Docker
- A cloned repository on a VM or scheduled runner
- Any local filesystem path containing Terraform configuration

For example, all of these future usage patterns should be valid:

```bash
terradrift scan
terradrift scan --directory ./terraform/prod
terradrift scan --directory ../company-infra/environments/prod
terradrift scan --directory /workspace/infrastructure/aws/prod
```

For CI usage, the recommended pattern is usually to run TerraDrift in the same job after checking out the Terraform repository you want to scan.

## Current CLI usage

```bash
terradrift scan
terradrift scan --directory ./terraform/prod
terradrift scan -d ./terraform/prod
terradrift scan -d ./terraform/prod --output json
terradrift scan -d ./terraform/prod --output junit
terradrift scan -d ./terraform/prod --output sarif
terradrift scan -d ./terraform/prod --output prometheus
terradrift scan-all --manifest terraform-roots.txt --concurrency 4 --output json
terradrift scan -d ./terraform/prod --timeout 2m --redact-paths
terradrift scan -d ./terraform/prod --workspace-root "$PWD"
terradrift scan -d ./terraform/prod --terraform-exec --output json
terradrift scan -d ./terraform/prod --terraform-exec --terraform-bin tofu
terradrift scan --config .terradrift.json
terradrift scan --config .terradrift.json --profile production
terradrift scan -d ./terraform/prod --notify slack --slack-webhook-url "$SLACK_WEBHOOK_URL"
terradrift scan -d ./terraform/prod --dashboard-html terradrift-report.html
terradrift scan -d ./terraform/prod --history-dir .terradrift-history --dashboard-html terradrift-report.html
terradrift scan -d ./terraform/prod --policy-command conftest --policy-arg test --policy-arg -
terradrift scan -d ./terraform/prod --cost-command infracost --cost-arg breakdown --cost-arg --format=json
terradrift init
```

If `--directory` is omitted, TerraDrift scans the current working directory.

`scan-all` reads one Terraform root per line from a manifest. Blank lines and `#` comments are ignored, and relative roots resolve from the manifest's directory. It runs roots with bounded concurrency and emits aggregate table or JSON output. The first multi-root pass intentionally excludes notifications, history, dashboards, policies, and cost enrichment.

Build a static cross-root dashboard index from recent history:

```bash
terradrift dashboard-index --history-dir .terradrift-history --output terradrift-index.html
```

Use `--discover <workspace>` to find roots containing `.tf` files. Repeat `--include` or `--exclude` with root-relative `filepath.Match` patterns; `.terraform` directories are always skipped. Use either `--manifest` or `--discover`, not both.

```text
# terraform-roots.txt
environments/development
environments/production
```

TerraDrift accepts any existing local directory at the CLI validation layer. When `--terraform-exec` is enabled, Terraform performs its own configuration validation and returns a scan failure if the selected directory is not usable Terraform configuration.

The `--timeout` flag applies a scan-level deadline to the current and future scan pipeline. The `--redact-paths` flag replaces local filesystem paths in scan output with `[REDACTED]`, which is useful for CI logs.

The `--workspace-root` flag evaluates symlinks and requires the selected Terraform directory to resolve inside the provided root, which is useful for constrained CI or hosted runner scenarios.

By default, TerraDrift still emits the bootstrap no-drift report. Use `--terraform-exec` to run the Terraform-compatible CLI flow: `init`, `plan -refresh-only -detailed-exitcode`, and `show -json`. `--terraform-bin` selects the executable, defaulting to `terraform`; set it to `tofu` for OpenTofu. The executable must be available on `PATH`.

Terraform-backed scans create `.terradrift-scan.lock` in the selected root to prevent overlap. The lock is removed when the scan exits; after a crash, remove it only after confirming no scan is still active.

The `terradrift init` command writes a starter `.terradrift.json` file with safe local defaults for repeated local or CI usage. Config files can also define optional scan settings such as `terraform_exec`, `terraform_bin`, `workspace_root`, `notify`, `slack_webhook_url`, `teams_webhook_url`, `webhook_url`, `dashboard_html`, `history_dir`, `policy_command`, `policy_args`, `cost_command`, `cost_args`, and `remediation_runbooks`; explicit CLI flags always take precedence.

Use [`docs/terradrift.schema.json`](docs/terradrift.schema.json) as the JSON Schema reference for editor and CI validation.

Use `profiles` for standalone development, staging, and production configurations. Select one with `--profile`; profile values do not inherit top-level settings.

```json
{
  "profiles": {
    "production": {
      "directory": "./terraform/prod",
      "output": "json",
      "terraform_exec": true,
      "redact_paths": true
    }
  }
}
```

Slack notifications are available with `--notify slack --slack-webhook-url "$SLACK_WEBHOOK_URL"`. Microsoft Teams notifications are available with `--notify teams --teams-webhook-url "$TEAMS_WEBHOOK_URL"`. Generic HTTPS webhooks are available with `--notify webhook --webhook-url "$WEBHOOK_URL"`. Notification messages use concise summaries and avoid including local filesystem paths or webhook secrets.

Static dashboard output is available with `--dashboard-html <path>`. This writes an escaped local HTML report that can be archived by CI or served by your own internal tooling. Historical JSON report storage is available with `--history-dir <directory>`; files are written with restrictive permissions and recent history is included in dashboard output when both flags are used.

Default table output:

```text
TerraDrift scan initialized
Status: no_drift
Terraform directory: /absolute/path/to/terraform/prod
Resources checked: 0
Changed resources: 0
```

JSON output is available for automation:

```json
{
  "status": "no_drift",
  "directory": "/absolute/path/to/terraform/prod",
  "total_resources_checked": 0,
  "total_changed_resources": 0,
  "resource_changes": [],
  "started_at": "2026-07-22T00:00:00Z",
  "completed_at": "2026-07-22T00:00:00Z"
}
```

JUnit XML output is available for CI test reporting. A detected drift result is reported as one failing `terradrift` test case.

SARIF output is available for code-scanning dashboards. Each changed resource is emitted as an error-level `terradrift.drift` result without a local filesystem location.

Prometheus text output exposes scan status, duration, and resource counts. It deliberately omits directory and resource labels to avoid exposing local paths or creating unbounded label cardinality.

Serve local scan history with a loopback-only API:

```bash
terradrift serve --history-dir .terradrift-history
```

`GET /reports` returns recent history as JSON and `GET /` renders an escaped dashboard. The server accepts only loopback listener addresses and has no write endpoints.

Without `--terraform-exec`, TerraDrift emits a bootstrap no-drift report after validating the selected directory.

## Installation and local development

### Requirements

- Go 1.26 or newer
- Docker, for container builds
- `golangci-lint`, for local linting

### Download dependencies

```bash
go mod download
```

### Run from source

```bash
go run ./cmd/terradrift scan
```

### Build the binary

```bash
make build
```

The binary is written to:

```text
bin/terradrift
```

### Run tests and checks

```bash
make fmt
make vet
make test
make test-race
make lint
make vuln
```

Tests do not require Terraform, cloud credentials, or network access. The vulnerability scan requires access to Go's vulnerability database.

## Scheduled scan examples

Reusable scheduled-run templates are available for:

- GitHub Actions: `examples/github-actions/terradrift-scheduled.yml`
- Cron or VM runners: `examples/cron/terradrift.cron`

Review and pin the TerraDrift, Terraform, provider, and module versions before using these examples in production. Keep cloud credentials and webhook URLs in CI secrets or a secret manager.

## Docker

Build the image:

```bash
make docker-build
```

The current runtime image intentionally does not install Terraform. To use `--terraform-exec` in Docker, build a derived image that installs Terraform or mount/provide a trusted Terraform binary on `PATH`. Pin Terraform, provider, and module versions in CI for repeatable drift results.

## Terraform caching

For scheduled CI scans, set `TF_PLUGIN_CACHE_DIR` to a job cache directory and cache it using a key that includes the Terraform lockfile hash and platform. Keep `.terraform.lock.hcl` committed; invalidate the cache when it changes. Do not share plugin caches between trust boundaries.

## Terraform execution flow

When `--terraform-exec` is provided, `terradrift scan` performs this flow:

1. Validate the Terraform directory.
2. Run `terraform init`.
3. Run `terraform plan -refresh-only -detailed-exitcode -out <planfile>`.
4. Run `terraform show -json <planfile>`.
5. Parse the JSON plan.
6. Produce a TerraDrift drift report.
7. Return clear exit codes for no drift, drift detected, and scan failure.

The CLI reserves these exit codes for automation-friendly workflows:

| Exit code | Meaning |
| ---: | --- |
| `0` | Scan completed successfully with no drift. |
| `1` | Scan failed before producing a reliable result. |
| `2` | Scan completed successfully and drift was detected. |

## Feature ideas and improvement backlog

Recent drift-detection guidance emphasizes scheduled scans, clear notifications, human-reviewed remediation, policy guardrails, and cost visibility. Based on that landscape, useful next TerraDrift additions include:

- Scheduled CI examples for GitHub Actions, cron, and container runners so teams can detect drift within hours instead of relying on ad-hoc checks.

## Cost-impact enrichment

Use `--cost-command <command>` to run an external cost tool before output, history, dashboards, policies, and notifications are produced. TerraDrift passes the current scan report JSON on stdin and expects JSON on stdout in this shape:

```json
{
  "resource_costs": [
    {"address": "aws_instance.web", "monthly_delta": "+$12.34/mo"}
  ]
}
```

Cost tool output is bounded before parsing, command errors are redacted, and arguments must be passed explicitly with repeated `--cost-arg` flags. Matching `address` values are copied into each resource change as `cost_impact`.

See [cost adapter guidance](docs/COST_ADAPTERS.md) for the normalized custom-command contract and Infracost workflow.

## Remediation guidance

Each changed resource includes conservative remediation guidance based on Terraform plan actions. The guidance intentionally keeps a human in the loop and frames choices such as updating Terraform configuration, importing or syncing state, restoring deleted infrastructure, or reverting an out-of-band change only after review and approval.

Reports also include review-only reconciliation hints for imports, moved blocks, and configuration updates. TerraDrift never runs state commands or applies infrastructure changes.

Each finding has a conservative action-based risk level: replacement is `critical`, deletion `high`, creation or update `medium`, and other actions `low`. Use `--failure-severity high` or `failure_severity` in config to fail CI only for active drift at or above that threshold; leaving it empty preserves failure on any drift.

Terraform-backed reports include the CLI version, selected provider versions, and initialized module key/source/version inventory. Local module directories are intentionally omitted.

Each Terraform resource change is classified as `aws`, `azure`, or `gcp` from its provider metadata or resource type. Terraform value data, including account IDs, regions, tags, and potential secrets, is intentionally not parsed.

Use exact-address `ignore_rules` for temporary, auditable exceptions. Each rule requires an owner, reason, and future RFC3339 expiry. Ignored findings stay visible in reports and dashboards but do not fail the scan.

Route active findings by owner with `resource_owners` and `owner_webhooks`. Exact resource addresses override resource types; each owner webhook uses the same HTTPS-only webhook protections as normal notifications.

Set `notification_throttle` to `true` with `history_dir` to suppress unchanged active drift notifications. New, removed, or higher-risk findings still notify; the first scan always notifies.

Post scan summaries to a GitHub pull request with `github_repository`, `github_pr`, and `GITHUB_TOKEN`. The token is read only from the environment and requires `pull-requests: write` permission.

```bash
GITHUB_TOKEN="$GITHUB_TOKEN" terradrift scan --github-repository owner/repo --github-pr 42
```

Set `github_issue_after` to create one GitHub issue when the same active drift fingerprint reaches that many consecutive scans of the same history root. It requires `github_repository`, `history_dir`, `GITHUB_TOKEN`, and a value of at least `2`.

Upload a JSON report to a presigned HTTPS URL with `--artifact-url` or `artifact_url`. This supports cloud object-storage uploads without storing cloud credentials in TerraDrift. Artifact URLs use the same HTTPS, public-destination, no-proxy, and no-redirect protections as generic webhooks.

Create a review-only approval artifact for a JSON drift report, then attach it to later scan output with `--approval-file`. Approvals are bound to the active drift fingerprint and expiry; they never apply Terraform or modify state.

```bash
terradrift approve --report report.json --owner platform-team \
  --reason "approved maintenance" --expires-at 2026-08-01T00:00:00Z
terradrift scan --approval-file report.json.approval.json
```

Correlate drift with CloudTrail, Azure Activity Log, or GCP Audit Log through `--audit-command`; see [audit adapter guidance](docs/AUDIT_ADAPTERS.md).

For CI, set `allowed_commands` and `trusted_command_dirs` in a profile. Bare commands must resolve on `PATH`; absolute commands must be under a trusted directory. Commands containing shell syntax are rejected.

```json
{
  "resource_owners": {
    "aws_instance": "platform",
    "aws_instance.web": "web-team"
  },
  "owner_webhooks": {
    "web-team": "https://alerts.example.com/web"
  }
}
```

```json
{
  "ignore_rules": [
    {
      "address": "aws_instance.web",
      "owner": "platform-team",
      "reason": "approved maintenance window",
      "expires_at": "2026-08-01T00:00:00Z"
    }
  ]
}
```

Set `remediation_runbooks` in `.terradrift.json` to link resources to HTTPS runbooks. Type/action entries override type entries:

```json
{
  "remediation_runbooks": {
    "aws_instance": "https://runbooks.example.com/instances",
    "aws_s3_bucket/delete": "https://runbooks.example.com/bucket-deletion"
  }
}
```

## Policy-as-code hooks

Use `--policy-command <command>` to run an external policy tool after the scan report is written and before notifications are sent. TerraDrift passes the redacted scan report JSON on stdin and never invokes a shell implicitly; pass each argument explicitly with repeated `--policy-arg` flags. A non-zero policy exit fails the scan, and policy stdout/stderr included in errors is size-limited and redacted before display.

Example with Conftest-style stdin usage:

```bash
terradrift scan \
  --directory ./terraform/prod \
  --redact-paths \
  --policy-command conftest \
  --policy-arg test \
  --policy-arg -
```

A reusable Conftest policy pack for destructive and production replacement drift is available in [`examples/policy`](examples/policy/README.md).

## Notifications

Slack, Microsoft Teams, and generic HTTPS webhook notifications can send a concise drift summary after the scan report is written, for example:

```text
Terraform drift scan completed
Status: drift_detected
Resources checked: 124
Changed resources: 3
```

Use CI secrets or environment variables for webhook URLs:

```bash
terradrift scan \
  --directory ./terraform/prod \
  --notify slack \
  --slack-webhook-url "$SLACK_WEBHOOK_URL"

terradrift scan \
  --directory ./terraform/prod \
  --notify teams \
  --teams-webhook-url "$TEAMS_WEBHOOK_URL"

terradrift scan \
  --directory ./terraform/prod \
  --notify webhook \
  --webhook-url "$WEBHOOK_URL"
```

Slack, Teams, and generic webhook URLs are redacted in notification errors, and notification payload tests verify that local filesystem paths and webhook secrets are not included. Generic webhook URLs must use HTTPS, cannot include user info, resolve only to allowed public IPs, and never follow redirects.

## Do you need to host TerraDrift?

For the CLI-first version, no hosting is required.

You can run TerraDrift from:

- A developer laptop
- GitHub Actions
- Another CI system
- A cron job on a VM
- A Docker container on a scheduled runner

No hosted service is required. For lightweight visibility, `--dashboard-html <path>` writes an escaped static HTML report that can be archived by CI or served by your own internal tooling. Use `--history-dir <directory>` to keep secure local JSON history and include recent scan trends in the static dashboard. A richer hosted service may be useful later for team visibility and long-term tracking.

## Example GitHub Actions usage

With Terraform execution enabled, a scheduled drift scan could look like this:

```yaml
name: Terraform Drift Scan

on:
  schedule:
    - cron: "0 * * * *"
  workflow_dispatch:

jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout infrastructure
        uses: actions/checkout@v7

      - name: Set up Terraform
        uses: hashicorp/setup-terraform@v3

      - name: Run TerraDrift
        run: terradrift scan --directory ./terraform/prod --output json
        env:
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
```

## High-level roadmap

1. Harden Terraform execution against more real-world provider and module edge cases.
2. Expand configuration support as repeated CI workflows mature.
3. Add more notification integrations with the same secret-safe defaults.
4. Improve dashboard/report styling and historical report workflows.
5. Continue adding redaction, security, performance, and large-fixture tests.

## Security considerations

- Treat Terraform configuration, state, plans, provider output, and logs as sensitive.
- Do not commit credentials, state files, generated plans, or logs containing secrets.
- Use least-privilege cloud credentials for drift detection.
- Do not log Slack webhook URLs or provider credentials.
- Review Terraform modules and providers before scanning untrusted code.
- Terraform contacts cloud APIs only when `--terraform-exec` is supplied.
- See [SECURITY.md](SECURITY.md) for vulnerability reporting guidance.

## Contributing

Contributions are welcome. Please keep changes focused, add tests for new behavior, and run the standard checks before opening a pull request:

```bash
make fmt
make vet
make test
make test-race
make lint
```

## License

TerraDrift is released under the MIT License. See [LICENSE](LICENSE).
