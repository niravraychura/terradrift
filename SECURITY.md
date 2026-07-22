# Security Policy

## Supported versions

TerraDrift is pre-1.0 and under active development. Security fixes are applied to the default branch until a formal release policy is published.

## Reporting a vulnerability

Please report suspected vulnerabilities privately by opening a GitHub security advisory for this repository. If advisories are unavailable, contact the maintainer directly before publishing details.

Include:

- Affected TerraDrift version or commit
- Reproduction steps
- Potential impact
- Any suggested mitigation

## Current security posture

By default, the CLI validates local directory input only and emits a bootstrap report. When `--terraform-exec` is explicitly enabled, TerraDrift executes Terraform locally and may contact cloud APIs through Terraform providers.

Future Terraform execution must treat Terraform modules, providers, plans, state files, and logs as potentially sensitive. Contributors should avoid adding behavior that prints secrets, commits state files, or executes untrusted Terraform without explicit user action.

Terraform execution will expand TerraDrift's trust boundary because `terraform init` can download modules and providers, `terraform plan -refresh-only` can contact cloud APIs, and plan/state-derived data can contain sensitive infrastructure values. Future runner work must use context timeouts, clean up temporary plan files, avoid logging raw Terraform output by default, and redact sensitive values before displaying diagnostics or sending notifications.

Slack notifications must be configured with secret storage such as CI secrets or environment variables. Do not commit webhook URLs. TerraDrift redacts webhook URLs in notification errors and sends concise Slack summaries that avoid local filesystem paths.
