# TerraDrift

[![CI](https://github.com/niravraychura/terradrift/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/niravraychura/terradrift/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/niravraychura/terradrift?include_prereleases&sort=semver)](https://github.com/niravraychura/terradrift/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

TerraDrift is an open-source, self-hosted Terraform drift detection tool. It is being built to help teams identify infrastructure changes that happened outside their normal Terraform workflow and surface those changes in a clear, automation-friendly way.

> [!WARNING]
> Without `--terraform-exec`, `terradrift scan` emits a bootstrap placeholder report (not a real drift result) and prints a warning. Use `--terraform-exec` for production CI. Attribute values are paths-only in history/policy/notifications unless `--attribute-values` is set; sensitive values stay redacted.

## Why TerraDrift exists

Terraform is the source of truth for many infrastructure teams, but real environments can still drift when resources are changed manually, by emergency scripts, by provider-side defaults, or by systems outside Terraform. TerraDrift aims to provide a simple self-hosted workflow for detecting that drift without requiring a hosted SaaS platform.

The long-term goal is to make drift detection easy to run from a developer laptop, CI pipeline, scheduled job, or private automation runner, then publish clear results to humans and automation systems.

## Current status

TerraDrift is a production-usable CLI for self-hosted drift detection. It includes:

- `scan` and `scan-all` with Terraform/OpenTofu execution (`--terraform-exec`), refresh-only and normal plan modes
- Workspace selection plus `-var` / `-var-file` passthrough
- Table and JSON reports with attribute-level diffs (safe values; secrets redacted)
- Policy publish gate, cost/audit adapters, Slack/Teams/webhook/GitHub notifications
- History, static dashboards, JUnit/SARIF/Prometheus outputs, severity gates
- Multi-root manifests with concurrent scans and shared delivery options
- Secret-safe defaults, size budgets, local scan locks, and CI/release hardening

Bootstrap (no Terraform) remains available for dry wiring checks only; prefer `--terraform-exec` for real results.

## Roadmap and releasing

- Product direction and out-of-scope items: [docs/ROADMAP.md](docs/ROADMAP.md)
- Release cycle (`dev` → `main` → `v*` tag): [docs/RELEASE.md](docs/RELEASE.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Contributing (PRs target **`dev`**): [CONTRIBUTING.md](CONTRIBUTING.md)
- Security reporting: [SECURITY.md](SECURITY.md)

Install binaries and container images from [GitHub Releases](https://github.com/niravraychura/terradrift/releases) once a `v*` tag is published.

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
terradrift scan-all --manifest terraform-roots.txt --incremental-state .terradrift-scan-state.json
terradrift scan -d ./terraform/prod --timeout 2m --redact-paths
terradrift scan -d ./terraform/prod --workspace-root "$PWD"
terradrift scan -d ./terraform/prod --terraform-exec --output json
terradrift scan -d ./terraform/prod --terraform-exec --plan-mode refresh-only
terradrift scan -d ./terraform/prod --terraform-exec --plan-mode normal
terradrift scan -d ./terraform/prod --terraform-exec --terraform-bin tofu
terradrift scan --config .terradrift.json
terradrift scan --config .terradrift.json --profile production
terradrift scan -d ./terraform/prod --notify slack --slack-webhook-url "$SLACK_WEBHOOK_URL"
terradrift scan -d ./terraform/prod --dashboard-html terradrift-report.html
terradrift scan -d ./terraform/prod --history-dir .terradrift-history --dashboard-html terradrift-report.html
terradrift scan -d ./terraform/prod --history-dir .terradrift-history --history-compressed --audit-log .terradrift-audit.jsonl
terradrift scan -d ./terraform/prod --policy-command conftest --policy-arg test --policy-arg -
terradrift scan -d ./terraform/prod --cost-command infracost --cost-arg breakdown --cost-arg --format=json
terradrift init
terradrift init --directory ./terraform/prod --terraform-exec --redact-paths --history-dir .terradrift-history
```

If `--directory` is omitted, TerraDrift scans the current working directory.

`scan-all` accepts a **text** or **JSON** manifest. Text manifests list one Terraform root per line (blank lines and `#` comments ignored; relative roots resolve from the manifest directory). JSON manifests use `version: 1` and may set per-root `profile`, `plan_mode`, `workspace`, `var_files`, and `vars` so mono-repos are not forced into one global flag set. Named `profile` values require `--config`. It runs roots with bounded concurrency and emits aggregate table or JSON output. `--incremental-state` is opt-in and retries only roots that previously drifted or failed; omit it for full coverage.

**`scan-all` delivery subset (production-usable):** `--history-dir`, `--dashboard-html`, `--notify` (plus webhook URL flags), `--policy-command`, `--cost-command`, `--audit-command`, `--attribute-values`, workspace/var-file defaults, and `--failure-severity` (gates exit code `2` for refresh-only drift the same way as `scan`). Policy runs as a publish gate before that root's history, dashboard, and notifications. Concurrent roots serialize history/dashboard writes.

**Not on `scan-all` yet** (use `terradrift scan` per root): baselines/owners/runbooks, GitHub PR/issue summaries, `--artifact-url`, `--audit-log`, notification throttle, approvals. Prefer `terradrift dashboard-index` for multi-root HTML; a shared `--dashboard-html` path is overwritten by the last successful root (and warns when `concurrency > 1`).

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

```json
{
  "version": 1,
  "roots": [
    {"directory": "terraform/dev", "profile": "development", "var_files": ["dev.tfvars"]},
    {"directory": "terraform/prod", "plan_mode": "refresh-only", "workspace": "prod", "var_files": ["prod.tfvars"]}
  ]
}
```

See [examples/multi-root](examples/multi-root) and [examples/github-actions/terradrift-scheduled-multi-root.yml](examples/github-actions/terradrift-scheduled-multi-root.yml) for a scheduled multi-root + Slack + high-severity gate.

### `scan` flag groups

| Group | Flags |
| --- | --- |
| Core | `--directory`, `--output`, `--timeout`, `--terraform-exec`, `--terraform-bin`, `--plan-mode`, `--workspace`, `--var-file`, `--var`, `--config`, `--profile`, `--failure-severity`, `--workspace-root`, `--redact-paths`, `--lock-backend`, `--skip-terraform-init`, `--attribute-values` |
| Delivery | `--history-dir`, `--history-retention`, `--history-compressed`, `--dashboard-html`, `--notify`, webhook URLs, `--artifact-url`, `--audit-log`, GitHub summary flags, `--approval-file` |
| Enrichment | `--policy-command`, `--policy-arg`, `--cost-command`, `--cost-arg`, `--audit-command`, `--audit-arg` |

`terradrift scan --help` lists the same groups in the command long description.

TerraDrift accepts any existing local directory at the CLI validation layer. When `--terraform-exec` is enabled, Terraform performs its own configuration validation and returns a scan failure if the selected directory is not usable Terraform configuration.

The `--timeout` flag applies a scan-level deadline to the current and future scan pipeline. The `--redact-paths` flag replaces local filesystem paths in scan output with `[REDACTED]`, which is useful for CI logs.

Terraform command output, configuration files, persisted history, approvals, external-adapter input, notification responses, and uploaded report artifacts have explicit size limits. CLI JSON and dashboard output stream directly instead of buffering a second copy of the report.

The `--workspace-root` flag evaluates symlinks and requires the selected Terraform directory to resolve inside the provided root, which is useful for constrained CI or hosted runner scenarios.

By default, TerraDrift still emits the bootstrap no-drift report. With `--terraform-exec`, `--plan-mode refresh-only` is the default and runs `plan -refresh-only -detailed-exitcode`; it reports remote infrastructure drift and state reconciliation only. `--plan-mode normal` runs `plan -detailed-exitcode` and reports all proposed configuration, state, and remote-object reconciliation changes. `--terraform-bin` selects the executable, defaulting to `terraform`; set it to `tofu` for OpenTofu. The executable must be available on `PATH`.

`plan_mode` accepts `refresh-only` or `normal` in `.terradrift.json`, including standalone named profiles. Explicit `--plan-mode` takes precedence. Refresh-only reports use `no_drift` and `drift_detected`; normal reports use `no_changes` and `changes_detected` so normal configuration differences are not labelled drift.

To compare a result, run both commands against the same directory, workspace, variables, backend, credentials, and Terraform/OpenTofu executable:

```bash
terradrift scan -d ./terraform/prod --terraform-exec --plan-mode refresh-only --output json
terradrift scan -d ./terraform/prod --terraform-exec --plan-mode normal --output json
```

Normal-plan changes with no refresh-only drift usually mean unapplied configuration changes rather than out-of-band infrastructure drift.

Terraform-backed scans create `.terradrift-scan.lock` in the selected root to prevent overlap on a **single host**. The lock is a local `O_EXCL` file lock (`--lock-backend local` only). Runners that share a filesystem can share the same lock file; Redis/Postgres distributed lock backends are out of scope. The lock is removed when the scan exits; after a crash, remove `.terradrift-scan.lock` manually only after confirming no scan is still active. When `--workspace-root` is set, TerraDrift re-validates the resolved directory after acquiring the lock to harden symlink TOCTOU races before Terraform runs.

By default each Terraform-backed scan runs `terraform init`. Use `--skip-terraform-init` only when `.terraform` (providers and modules) is already valid for the selected root; skipping init with a stale or missing working directory will fail the scan. Use `--workspace` to `terraform workspace select` before plan, and repeatable `--var-file` / `--var` to pass Terraform variables into plan (also via config `workspace`, `var_files`, `vars`).

The `terradrift init` command writes a tailored `.terradrift.json` file with safe local defaults. Use its `--directory`, `--terraform-exec`, `--redact-paths`, and `--history-dir` flags to guide the initial configuration. Config files can also define optional scan settings such as `terraform_exec`, `terraform_bin`, `plan_mode`, `workspace`, `var_files`, `vars`, `workspace_root`, `attribute_values`, `notify`, `slack_webhook_url`, `teams_webhook_url`, `webhook_url`, `webhook_ca_cert`, `dashboard_html`, `history_dir`, `history_compressed`, `audit_log`, `policy_command`, `policy_args`, `cost_command`, `cost_args`, `baseline_rules`, and `remediation_runbooks`; explicit CLI flags always take precedence. Audit logs are JSON Lines with allowlisted metadata and redacted errors.

Use [`docs/terradrift.schema.json`](docs/terradrift.schema.json) as the JSON Schema reference for editor and CI validation.

Supported local and CI configuration examples are in [`examples/config`](examples/config/README.md).

See [architecture and report-stability guarantees](docs/ARCHITECTURE.md) before integrating report JSON with automation.

Use `profiles` for standalone development, staging, and production configurations. Select one with `--profile`; profile values do not inherit top-level settings.

```json
{
  "profiles": {
    "production": {
      "directory": "./terraform/prod",
      "output": "json",
      "terraform_exec": true,
      "plan_mode": "refresh-only",
      "redact_paths": true
    }
  }
}
```

Slack notifications are available with `--notify slack --slack-webhook-url "$SLACK_WEBHOOK_URL"`. Microsoft Teams notifications are available with `--notify teams --teams-webhook-url "$TEAMS_WEBHOOK_URL"`. Generic HTTPS webhooks are available with `--notify webhook --webhook-url "$WEBHOOK_URL"`. For enterprise TLS interception, pass `--webhook-ca-cert /path/to/ca.pem` (or `webhook_ca_cert` in config) so Slack, Teams, generic, and owner webhooks trust your custom CA. Notification messages use concise summaries and avoid including local filesystem paths or webhook secrets.

Static dashboard output is available with `--dashboard-html <path>`. This writes an escaped local HTML report that can be archived by CI or served by your own internal tooling. Historical JSON report storage is available with `--history-dir <directory>`; files are written with restrictive permissions and recent history is included in dashboard output when both flags are used.

Default table output:

```text
TerraDrift scan initialized
Status: changes_detected
Plan mode: normal
Scan ID: 0a0cb23d-f342-4ce5-8517-03ba05aee949
Terraform directory: /absolute/path/to/terraform/prod
Resources checked: 144
Changed resources: 2

CRITICAL  delete,create  module.ecs.aws_ecs_task_definition.td
  reason: replace_because_cannot_update
  cpu: "256" -> "512"

MEDIUM  update  module.alb.aws_lb.main
  idle_timeout: 60 -> 120
  tags.Environment: "staging" -> "dev"
```

Attribute diffs use Terraform plan before/after values with security controls:

- Terraform `before_sensitive` / `after_sensitive` and name heuristics (`password`, `token`, `connection_string`, `db_url`, `user_data`, and similar) are shown as `[REDACTED]`.
- Large strings and encoded objects over 200 characters are summarized as `[changed, NB]` instead of partial dumps.
- Human table/JSON **stdout** may include these safe values.
- By default, history, uploaded artifacts, policy stdin, dashboards, and notifications use **paths only** (attribute paths without Before/After). Pass `--attribute-values` (or config `attribute_values: true`) to include the same safe/redacted values in those outputs. Secrets are never persisted in cleartext.

JSON output is available for automation and includes the same `attribute_changes` on each resource:

```json
{
  "status": "no_drift",
  "plan_mode": "refresh-only",
  "directory": "/absolute/path/to/terraform/prod",
  "total_resources_checked": 0,
  "resources_checked_exact": false,
  "total_changed_resources": 0,
  "resource_changes": [],
  "started_at": "2026-07-22T00:00:00Z",
  "completed_at": "2026-07-22T00:00:00Z"
}
```

JUnit XML output is available for CI test reporting. A detected drift or normal-plan change result is reported as one failing `terradrift` test case.

SARIF output is available for code-scanning dashboards. Refresh-only findings use `terradrift.drift`; normal-plan findings use `terradrift.change`, both without a local filesystem location.

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

The current runtime image intentionally does not install Terraform. To use `--terraform-exec` in Docker, build a derived image that installs Terraform or OpenTofu, or mount a trusted binary on `PATH`. Example derived image:

```dockerfile
FROM ghcr.io/niravraychura/terradrift:latest
USER root
RUN apk --no-cache add curl unzip \
  && curl -fsSLo /tmp/terraform.zip https://releases.hashicorp.com/terraform/1.10.5/terraform_1.10.5_linux_amd64.zip \
  && unzip /tmp/terraform.zip -d /usr/local/bin \
  && rm /tmp/terraform.zip \
  && chmod 0755 /usr/local/bin/terraform
USER terradrift:terradrift
```

Pin TerraDrift, Terraform/OpenTofu, provider, and module versions in CI for repeatable drift results. Do not bake cloud credentials into the image.

## Releases

Pushing an existing `v*` tag builds Linux amd64 and macOS arm64 archives, publishes SHA-256 checksums, an image SBOM, provenance attestations, and a keyless-signed GHCR runtime image. CI scans the final runtime image and rejects AGPL or GPL-3 licenses.

## Terraform caching

For scheduled CI scans, set `TF_PLUGIN_CACHE_DIR` to a job cache directory and cache it using a key that includes the Terraform lockfile hash and platform. Keep `.terraform.lock.hcl` committed; invalidate the cache when it changes. Do not share plugin caches between trust boundaries.

## Terraform execution flow

When `--terraform-exec` is provided, `terradrift scan` performs this flow:

1. Validate the Terraform directory.
2. Run `terraform init -input=false -backend=true -lockfile=readonly` unless `--skip-terraform-init` is set. A committed `.terraform.lock.hcl` is required when init runs; TerraDrift never upgrades providers or rewrites the lockfile. Skip init only when `.terraform` is already valid for the selected root.
3. Run `terraform plan -refresh-only -detailed-exitcode -out <planfile>` for refresh-only mode, or `terraform plan -detailed-exitcode -out <planfile>` for normal mode.
4. Run `terraform show -json <planfile>`.
5. Parse the JSON plan.
6. Produce a TerraDrift report.
7. Return clear exit codes for no changes, detected drift or changes, and scan failure.

The CLI reserves these exit codes for automation-friendly workflows:

| Exit code | Meaning |
| ---: | --- |
| `0` | Scan completed successfully with no drift or normal-plan changes (or active findings below `--failure-severity`). |
| `1` | Scan failed before producing a reliable result (including policy publish-gate failure). |
| `2` | Scan completed successfully and drift or normal-plan changes were detected at/above the failure severity threshold. |

Example: with `--failure-severity high`, medium updates exit `0`, but a replacement (`critical`) exits `2`.

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

Terraform-backed reports include the CLI version, selected provider versions, initialized module key/source/version inventory, and managed instance inventory from `prior_state`. Module inventory metadata identifies initialized modules only; it does not prove that module resources were counted or drifted. `resources_checked_exact` is true only when prior state supplied a root module; otherwise the count is an estimate from plan entries. Local module directories are intentionally omitted.

Each Terraform resource change is classified as `aws`, `azure`, or `gcp` from its provider metadata or resource type. Terraform value data, including account IDs, regions, tags, and potential secrets, is intentionally not parsed.

Use exact-address `baseline_rules` for accepted known drift and `ignore_rules` for temporary exceptions. Both require an owner, reason, and future RFC3339 expiry; explicit ignore rules take precedence when both match. Their metadata remains in reports and history as an audit trail. Ignored findings stay visible in reports and dashboards but do not fail the scan.

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

For CI, set absolute `allowed_commands` and `trusted_command_dirs` in a profile (see `examples/config/ci.json`). Resolved commands, including bare names, must be under a trusted directory; commands containing shell syntax are rejected. Empty allowlists mean **local trust only**—fine on a laptop, unsafe for shared CI without pinning both fields.

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

Use `--policy-command <command>` to run an external policy tool as a **publish gate** after stdout is written and **before** history, dashboard HTML, artifact upload, and notifications. On policy failure the scan returns an error immediately and does not persist or notify. TerraDrift passes the (paths-only by default) scan report JSON on stdin and never invokes a shell implicitly; pass each argument explicitly with repeated `--policy-arg` flags. A non-zero policy exit fails the scan, and policy stdout/stderr included in errors is size-limited (fail-closed on truncation) and redacted before display.

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
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1

      - name: Set up Terraform
        uses: hashicorp/setup-terraform@dfe3c3f87815947d99a8997f908cb6525fc44e9e # v4.0.1

      - name: Run TerraDrift
        run: terradrift scan --directory ./terraform/prod --terraform-exec --output json
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
- Use encrypted remote state with state locking; do not copy production state into CI workspaces.
- Prefer read-only cloud credentials for refresh-only drift scans and scope them to the scanned environment.
- Store webhook URLs, GitHub tokens, artifact URLs, and cloud audit credentials in a secret manager or CI secret store.
- Do not commit credentials, state files, generated plans, or logs containing secrets.
- Use least-privilege cloud credentials for drift detection.
- Do not log Slack webhook URLs or provider credentials.
- Review Terraform modules and providers before scanning untrusted code.
- Terraform contacts cloud APIs only when `--terraform-exec` is supplied.
- See [SECURITY.md](SECURITY.md) for vulnerability reporting guidance.
- See [drift-scan IAM and secret-scanning guidance](docs/DRIFT_SCAN_IAM.md) before configuring cloud credentials.

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

## Maintainers / agents

- Humans: [CONTRIBUTING.md](CONTRIBUTING.md)
- Coding agents / Cursor defaults: [AGENTS.md](AGENTS.md) and [`.cursor/rules/`](.cursor/rules/)
- GitHub product packaging checklist: [docs/GITHUB_PRODUCT_SETUP.md](docs/GITHUB_PRODUCT_SETUP.md)
