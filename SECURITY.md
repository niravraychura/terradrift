# Security Policy

## Supported versions

TerraDrift is pre-1.0 and under active development. Security fixes land on `dev` first, then `main` through the normal promotion path, until a formal supported-release policy is published.

## Reporting a vulnerability

Please report suspected vulnerabilities privately by opening a GitHub security advisory for this repository. If advisories are unavailable, contact the maintainer directly before publishing details.

Include:

- Affected TerraDrift version or commit
- Reproduction steps
- Potential impact
- Any suggested mitigation

## Trust boundary

TerraDrift is a **self-hosted CLI**. It assumes a trusted runner and trusted Terraform configuration.

When `--terraform-exec` is enabled, TerraDrift runs Terraform/OpenTofu locally. That expands the trust boundary because:

- `terraform init` can download providers and modules
- `terraform plan` can contact cloud APIs using credentials available to the process
- plan and state data can contain sensitive infrastructure values

Without `--terraform-exec`, TerraDrift only validates the directory and emits a bootstrap placeholder report (with a warning). That mode is for wiring checks, not production drift detection.

Do not commit cloud credentials, webhook URLs, or `GITHUB_TOKEN` values. Keep them in CI secrets or a secret manager.

## Current security posture

### Execution and filesystem

- Scan-level `--timeout` bounds Terraform and delivery work.
- Temporary plan files are created with restrictive permissions and cleaned up.
- `--redact-paths` replaces local filesystem paths in user-facing output.
- `--workspace-root` requires the scan directory to resolve inside a trusted root (symlink-aware) and re-validates after lock acquisition to reduce symlink TOCTOU risk.
- History, dashboard, and approval outputs reject direct symlink targets and use restrictive file permissions where applicable.
- I/O for Terraform output, config, history, adapters, notifications, and artifacts is size-bounded; overrun fails closed where parsers would otherwise see truncated data.

### Attribute values and secrets

- Changed attribute **paths** are always reported.
- Values are shown only when safe: Terraform sensitive marks, name heuristics (for example `password`, `token`, `connection_string`, `*_key`), and large blobs are redacted or summarized.
- By default, history, uploaded artifacts, policy stdin, dashboards, and notifications use **paths only**. `--attribute-values` / `attribute_values` may include the same safe/redacted scalars in those channels; secrets must never be persisted in cleartext.
- Errors and notification text are redacted before display (webhook URLs, common credential patterns, sensitive query parameters).

### Notifications and outbound HTTP

- Slack, Teams, generic webhooks, artifact upload, and GitHub issue/PR delivery share an SSRF-hardened HTTPS client: no proxy, no redirects, blocked private/loopback/link-local destinations, and explicit dial / TLS / overall timeouts.
- Optional `--webhook-ca-cert` (or `webhook_ca_cert`) loads a PEM CA bundle for enterprise TLS interception.
- `GITHUB_TOKEN` is read only from the environment and validated early when GitHub delivery is configured.

### Policy, adapters, and publish gate

- External policy, cost, and audit commands never use an implicit shell; pass arguments with repeated `--*-arg` flags.
- For CI, set both `allowed_commands` and `trusted_command_dirs`. Empty allowlists mean **local trust only**.
- Policy runs as a **publish gate**: on failure, TerraDrift does not write history, dashboards, artifacts, or notifications for that scan (stdout may already have been emitted).
- Adapter stdout/stderr capture fails closed when size budgets are exceeded.

### Locks

- Terraform-backed scans use a local `.terradrift-scan.lock` (`--lock-backend local` only) to prevent overlapping scans of the same root on a **single host**.
- Shared filesystems can share that lock file across runners on the volume. Redis/Postgres distributed backends are out of scope.
- If a lock already exists, TerraDrift reports the recorded PID and whether that process appears to be running. Remove a stale lock only after confirming no scan is active.

### Local API (`serve`)

- `terradrift serve` binds to loopback only and has no authentication. Do not expose it through a tunnel or public interface without your own front-door controls. Multi-tenant auth is out of scope.

### Supply chain and CI

- User-facing and CI workflows pin third-party GitHub Actions to immutable SHAs.
- Releases produce checksums, SBOM, provenance, and image scanning as configured in repository workflows.
- Prefer pinned Terraform/OpenTofu and committed `.terraform.lock.hcl`; TerraDrift init uses `-lockfile=readonly` and does not upgrade providers.

## Operator checklist

1. Use `--terraform-exec` (or config) for real scans; do not treat bootstrap output as drift truth.
2. Prefer `--redact-paths` and `--workspace-root` in CI.
3. Keep webhook URLs and tokens in secrets; never commit them.
4. Set `allowed_commands` and `trusted_command_dirs` for any policy/cost/audit adapters in CI.
5. Leave `--attribute-values` off unless you intentionally need safe values in persisted/automation channels.
6. Treat policy failure as a failed publish, not only a log line.
7. For multi-runner CI, do not assume the local file lock coordinates across hosts unless they share the lock path on a shared filesystem.
