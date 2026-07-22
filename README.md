# TerraDrift

TerraDrift is an open-source, self-hosted Terraform drift detection tool. It is being built to help teams identify infrastructure changes that happened outside their normal Terraform workflow and surface those changes in a clear, automation-friendly way.

> [!WARNING]
> TerraDrift is under active development. The current bootstrap release validates a local Terraform directory only. It does **not** run Terraform, detect drift, parse plans, send Slack messages, or provide a dashboard yet.

## Why TerraDrift exists

Terraform is the source of truth for many infrastructure teams, but real environments can still drift when resources are changed manually, by emergency scripts, by provider-side defaults, or by systems outside Terraform. TerraDrift aims to provide a simple self-hosted workflow for detecting that drift without requiring a hosted SaaS platform.

The long-term goal is to make drift detection easy to run from a developer laptop, CI pipeline, scheduled job, or private automation runner, then publish clear results to humans and automation systems.

## Current status

The first version of TerraDrift is a project foundation. It includes:

- A Go module and Cobra-based CLI named `terradrift`
- A `scan` command that defaults to the current directory and supports `--directory` / `-d`
- Directory validation and absolute path reporting
- Human-friendly table output and automation-friendly JSON output
- Documented exit codes for future CI drift workflows
- Structured logging foundations with `log/slog`
- Domain models for future drift reports
- A Terraform runner interface for future Terraform CLI integration
- Unit tests that do not require Terraform or cloud credentials
- Docker, Makefile, GitHub Actions CI, Dependabot, and security policy scaffolding

Actual Terraform execution is the next major implementation step.

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
```

If `--directory` is omitted, TerraDrift scans the current working directory.

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
  "resource_changes": null,
  "started_at": "2026-07-22T00:00:00Z",
  "completed_at": "2026-07-22T00:00:00Z"
}
```

Today, this only resolves the selected directory to an absolute path, verifies that it exists and is a directory, and emits a bootstrap no-drift report. Real Terraform execution is still planned next.

## Logging

The CLI supports a global `--log-level` flag:

```bash
terradrift --log-level debug scan --directory ./terraform/prod
```

Supported values:

- `debug`
- `info`
- `warn`
- `error`

The default log level is `info`.

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

## Docker

Build the image:

```bash
make docker-build
```

The current runtime image intentionally does not install Terraform yet. Terraform will be included, mounted, or otherwise discovered when command execution is implemented.

## Planned real drift detection flow

The next implementation should make `terradrift scan` perform this flow:

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

## Slack and notifications

Slack notifications are not implemented yet.

The intended future workflow is to generate a drift report and optionally send a concise summary to Slack, for example:

```text
🚨 Terraform drift detected
Environment: prod
Resources checked: 124
Changed resources: 3
```

A future command may look like:

```bash
terradrift scan \
  --directory ./terraform/prod \
  --notify slack \
  --slack-webhook-url "$SLACK_WEBHOOK_URL"
```

Notification work should include secret-safe logging, webhook redaction, tests for message formatting, and tests proving sensitive values are not printed.

## Do you need to host TerraDrift?

For the CLI-first version, no hosting is required.

You can run TerraDrift from:

- A developer laptop
- GitHub Actions
- Another CI system
- A cron job on a VM
- A Docker container on a scheduled runner

A hosted service or dashboard may be useful later for historical drift reports, team visibility, and long-term tracking, but that is intentionally not part of the initial CLI foundation.

## Example future GitHub Actions usage

Once Terraform execution is implemented, a scheduled drift scan could look like this:

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

1. Implement the Terraform CLI runner.
2. Add refresh-only plan execution and JSON plan parsing.
3. Convert Terraform plan output into TerraDrift report models.
4. Wire real drift outcomes to the documented exit codes.
5. Add config file support for repeated local and CI usage.
6. Add a guided `terradrift init` setup command.
7. Add notification integrations such as Slack with secret-safe defaults.
8. Add redaction and security-focused tests.
9. Add Docker runtime support for Terraform execution.
10. Consider a self-hosted dashboard after the CLI workflow is stable.

## Security considerations

- Treat Terraform configuration, state, plans, provider output, and logs as sensitive.
- Do not commit credentials, state files, generated plans, or logs containing secrets.
- Use least-privilege cloud credentials for drift detection.
- Do not log Slack webhook URLs or provider credentials.
- Review Terraform modules and providers before scanning untrusted code.
- This bootstrap release does not contact cloud APIs or execute Terraform.
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
