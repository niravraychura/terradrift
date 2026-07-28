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
terradrift scan -d ./terraform/prod --timeout 2m --redact-paths
terradrift scan -d ./terraform/prod --workspace-root "$PWD"
terradrift scan -d ./terraform/prod --terraform-exec --output json
terradrift scan --config .terradrift.json
terradrift scan -d ./terraform/prod --notify slack --slack-webhook-url "$SLACK_WEBHOOK_URL"
terradrift scan -d ./terraform/prod --dashboard-html terradrift-report.html
terradrift scan -d ./terraform/prod --history-dir .terradrift-history --dashboard-html terradrift-report.html
terradrift scan -d ./terraform/prod --policy-command conftest --policy-arg test --policy-arg -
terradrift scan -d ./terraform/prod --cost-command infracost --cost-arg breakdown --cost-arg --format=json
terradrift init
```

If `--directory` is omitted, TerraDrift scans the current working directory.

TerraDrift accepts any existing local directory at the CLI validation layer. When `--terraform-exec` is enabled, Terraform performs its own configuration validation and returns a scan failure if the selected directory is not usable Terraform configuration.

The `--timeout` flag applies a scan-level deadline to the current and future scan pipeline. The `--redact-paths` flag replaces local filesystem paths in scan output with `[REDACTED]`, which is useful for CI logs.

The `--workspace-root` flag evaluates symlinks and requires the selected Terraform directory to resolve inside the provided root, which is useful for constrained CI or hosted runner scenarios.

By default, TerraDrift still emits the bootstrap no-drift report. Use `--terraform-exec` to run the Terraform CLI flow: `terraform init`, `terraform plan -refresh-only -detailed-exitcode`, and `terraform show -json`. This requires Terraform to be installed and available on `PATH`.

The `terradrift init` command writes a starter `.terradrift.json` file with safe local defaults for repeated local or CI usage. Config files can also define optional scan settings such as `terraform_exec`, `workspace_root`, `notify`, `slack_webhook_url`, `teams_webhook_url`, `webhook_url`, `dashboard_html`, `history_dir`, `policy_command`, `policy_args`, `cost_command`, and `cost_args`; explicit CLI flags always take precedence.

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

## Remediation guidance

Each changed resource includes conservative remediation guidance based on Terraform plan actions. The guidance intentionally keeps a human in the loop and frames choices such as updating Terraform configuration, importing or syncing state, restoring deleted infrastructure, or reverting an out-of-band change only after review and approval.

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
