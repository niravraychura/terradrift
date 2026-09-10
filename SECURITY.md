# Security Policy

## Supported versions

TerraDrift is pre-1.0. Security fixes land on `dev` first, then `main` through the normal promotion path. Until 1.0, **only the latest tagged release on `main`** (and `main` itself) is supported for security fixes. See [docs/RELEASE.md](docs/RELEASE.md) for the release cycle.

## Reporting a vulnerability

Please report suspected vulnerabilities **privately** via [GitHub private vulnerability reporting](https://github.com/niravraychura/terradrift/security/advisories/new) (Settings → Code security) or a draft security advisory. Do not file public issues with secrets or exploit details. If private reporting is unavailable, contact the maintainer directly before publishing details.

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

Without `--terraform-exec`, TerraDrift only validates the directory and emits a bootstrap placeholder report (with a warning). That mode is for wiring checks, not production drift detection. In GitHub Actions (`GITHUB_ACTIONS=true`) or when `TERRADRIFT_REQUIRE_EXEC` is set, omitting `--terraform-exec` fails the scan.

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
- GitHub PR/issue delivery honors `GITHUB_API_URL` (HTTPS, no userinfo; GitHub Actions sets this on GHES/GHEC). That **one** API host may resolve to a private IP. Generic `--notify webhook` destinations stay blocked.
- Optional `--webhook-ca-cert` (or `webhook_ca_cert`) loads a PEM CA bundle for enterprise TLS interception.
- `GITHUB_TOKEN` is read only from the environment and validated early when GitHub delivery is configured.
- `--skip-if-open-pr` lists open pull requests and their files; it skips instead of planning when a file path sits under the Terraform root. It does not report `no_drift`.

### Policy, adapters, and publish gate

- External policy, cost, and audit commands never use an implicit shell; pass arguments with repeated `--*-arg` flags.
- For CI, set both `allowed_commands` and `trusted_command_dirs`. Empty allowlists mean **local trust only**. GitHub Actions (`GITHUB_ACTIONS=true`) and `TERRADRIFT_REQUIRE_EXEC` reject policy/cost/audit adapters unless both lists are set.
- Policy runs as a **publish gate**: on failure, TerraDrift does not write history, dashboards, artifacts, or notifications for that scan. **Stdout is already emitted** before policy, so treat a non-zero exit as the gate — do not assume a printed report means policy passed.
- Adapter stdout/stderr capture fails closed when size budgets are exceeded.

### Locks

- Terraform-backed scans use a local `.terradrift-scan.lock` (`--lock-backend local` only) to prevent overlapping scans of the same root on a **single host**.
- Shared filesystems can share that lock file across runners on the volume. Redis/Postgres distributed backends are out of scope.
- If a lock already exists, TerraDrift reports the recorded PID and whether that process appears to be running. Remove a stale lock only after confirming no scan is active.
- `terraform plan` waits for the **remote state lock** (`-lock-timeout`, default 10m). Do not `force-unlock`. `--state-lock=false` skips that lock and can race with apply; it is opt-in for scheduled drift only.
- `--plan-file` reuses a trusted local plan. The path must be a regular file (not a symlink), size-bounded, and inside `--workspace-root` when that flag is set. TerraDrift still runs `terraform show -json` (requires `--terraform-exec`) and never applies. Do not upload plan files as CI artifacts.

### Plan JSON honesty

- Incomplete Terraform 1.14+ plans (`complete=false`, `errored=true`, or non-empty `deferred_changes`) fail the scan. They are never reported as `no_drift`.

### Local API (`serve`)

- `terradrift serve` binds to loopback only and has no authentication. Do not expose it through a tunnel or public interface without your own front-door controls. Multi-tenant auth is out of scope.

### Supply chain and CI

- User-facing and CI workflows pin third-party GitHub Actions to immutable SHAs. The official TerraDrift Action always passes `--terraform-exec`.
- GitHub **secret scanning** and **push protection** are enabled for this public repository.
- **CodeQL** runs via `.github/workflows/codeql.yml` (PRs, pushes to `main`/`dev`, weekly); findings appear under Security → Code scanning.
- Releases produce checksums, SBOM, provenance, and image scanning as configured in repository workflows.
- Prefer pinned Terraform/OpenTofu and committed `.terraform.lock.hcl`; TerraDrift init uses `-lockfile=readonly` and does not upgrade providers.

## Operator checklist

1. Use `--terraform-exec` (or config) for real scans; do not treat bootstrap output as drift truth. CI (`GITHUB_ACTIONS` / `TERRADRIFT_REQUIRE_EXEC`) fails closed without it.
2. Prefer `--redact-paths` and `--workspace-root` in CI.
3. Keep webhook URLs and tokens in secrets; never commit them.
4. Set `allowed_commands` and `trusted_command_dirs` for any policy/cost/audit adapters in CI. CI fails closed if those lists are empty while an adapter is configured.
5. Leave `--attribute-values` off unless you intentionally need safe values in persisted/automation channels.
6. Treat policy failure as a failed publish, not only a log line.
7. For multi-runner CI, do not assume the local file lock coordinates across hosts unless they share the lock path on a shared filesystem.
8. Only scan Terraform/OpenTofu roots and cloud accounts you are authorized to plan. TerraDrift uses whatever credentials the process already has.
