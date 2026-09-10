# Changelog

All notable changes to TerraDrift are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.1] - 2026-09-10

### Added

- GitLab CI scheduled scan example (`examples/gitlab-ci/`) (#110)
- GitHub Marketplace listing for the official Action ([TerraDrift Scan](https://github.com/marketplace/actions/terradrift-scan)) (#111)
- Homebrew tap `niravraychura/tap` (`brew install niravraychura/tap/terradrift`) (#109, #129)

### Changed

- README / SECURITY: trademarks, no HashiCorp/OpenTofu/GitHub affiliation, operator is responsible for permission to plan (#131)
- Action Marketplace description: independent CLI, not affiliated with HashiCorp (#133)

## [0.4.0] - 2026-09-10

### Added

- `release.yml` Cosign-signs CLI tarballs (keyless OIDC bundles next to each archive) (#91)
- `docs/COMPARE.md` — vs `terraform plan`, inventory scanners, orchestrators, and rootsami/terradrift (#108)
- `scan-all --output junit` and `--output sarif` emit one aggregate artifact across roots (#86)
- Official GitHub Action (`action.yml`) that always passes `--terraform-exec`, fails if Terraform/OpenTofu is missing, and caches providers (#89)
- `--plan-file` / `plan_file` to reuse a trusted local Terraform plan (`show -json` only; still requires `--terraform-exec`) (#90)
- `--state-lock` / `--state-lock-timeout` (default 10m) and `state_lock` / `state_lock_timeout` config for Terraform remote state locking
- `TF_IN_AUTOMATION=1` and `-no-color` / `-input=false` on Terraform CLI invocations

### Changed

- README states this is a plan-based CLI (not the 2023 Terradrift server), that bootstrap exit 0 is not “no drift”, and shows a Terraform-backed scan transcript (#107)
- `scan-all` refuses shared `--dashboard-html`, `--artifact-url`, and `--github-pr` when more than one root would overwrite the same destination (#85)
- GitHub Actions / `TERRADRIFT_REQUIRE_EXEC` require `allowed_commands` and `trusted_command_dirs` when policy, cost, or audit adapters are set (#88)
- Docs state that stdout is emitted before the policy publish gate; treat the exit code as the gate, not the printed report (#87)
- GitHub Actions examples install a real release via `scripts/install.sh`, cache providers, and document OIDC / GitHub token permissions (#102)

### Fixed

- GitHub Actions (`GITHUB_ACTIONS=true`) and `TERRADRIFT_REQUIRE_EXEC` fail closed unless `--terraform-exec` is set (#83)
- `--notify github` is rejected; use `--github-pr` or `--github-issue-after` (#84)
- `--skip-terraform-init` fails if `.terraform` is missing or uninitialized (#98)
- Incomplete Terraform plans (`complete=false`, `errored=true`, or `deferred_changes`) fail instead of reporting no drift (#99)

## [0.3.0] - 2026-09-09

### Added

- `terradrift init` and example configs include `"$schema"` for editor validation
- OpenTofu GitHub Actions example (`examples/github-actions/terradrift-opentofu.yml`)
- Ignore/baseline addresses accept globs (`module.vpc.*`) in addition to exact matches
- GitHub PR comments are upserted (one TerraDrift comment per PR) instead of posting a new comment every scan
- Release archives for `linux_arm64` and `darwin_amd64`
- Checksum-verifying `scripts/install.sh` and documented `terradrift completion`
- Scan progress on stderr (`init` / `plan` / `show` / parse), with `--quiet` to suppress it
- `scan-all --output prometheus`
- Dashboard HTML styling and `dashboard-index` grouping by Terraform root directory

### Changed

- Prometheus scan metrics include a bounded `root_id` label (hash, never a filesystem path). Unlabeled series from v0.2.0 will not receive new samples.

## [0.2.0] - 2026-09-09

### Added

- `terradrift --version` / `-v` (release builds inject the tag via `-ldflags`)
- `scan-all` delivery parity with `scan`: ignore/baseline, owners/runbooks, approvals, GitHub PR/issue, artifact upload, audit-log, notification throttle, and config allowlists

### Fixed

- Refresh-only JSON/table reports now include attribute diffs when Terraform puts values on `resource_changes`, identity fields, or `relevant_attributes` instead of `resource_drift`

### Security

- Runtime container image upgrades Alpine OpenSSL with the base image so known critical/high CVEs are not shipped in GHCR

## [0.1.0] - 2026-08-19

First public tagged release of the self-hosted TerraDrift CLI.

### Added

- `scan` and `scan-all` with Terraform/OpenTofu execution (`--terraform-exec`), refresh-only and normal plan modes
- Workspace selection and `-var` / `-var-file` passthrough
- Table and JSON reports with attribute-level diffs (paths-only by default; secrets redacted)
- Policy publish gate, cost/audit adapters, Slack/Teams/webhook/GitHub notifications
- History, static dashboards, JUnit/SARIF/Prometheus outputs, severity gates
- Multi-root manifests with concurrent scans and shared delivery options
- CI, Dependabot, and release workflow (archives, checksums, SBOM, signed GHCR image)
- GitHub product packaging: release policy, contributing guide, issue/PR templates, Cursor/agent defaults, and roadmap docs

### Security

- Trusted-runner model documented in `SECURITY.md`
- SSRF-safe GitHub HTTP client, fail-closed truncated I/O, attribute value heuristics
- Size budgets and redaction defaults for user-facing output

[Unreleased]: https://github.com/niravraychura/terradrift/compare/v0.4.1...HEAD
[0.4.1]: https://github.com/niravraychura/terradrift/releases/tag/v0.4.1
[0.4.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.4.0
[0.3.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.3.0
[0.2.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.2.0
[0.1.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.1.0
