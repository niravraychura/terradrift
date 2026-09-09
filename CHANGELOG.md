# Changelog

All notable changes to TerraDrift are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `terradrift init` and example configs include `"$schema"` for editor validation
- OpenTofu GitHub Actions example (`examples/github-actions/terradrift-opentofu.yml`)
- Release archives for `linux_arm64` and `darwin_amd64`
- Checksum-verifying `scripts/install.sh` and documented `terradrift completion`

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

[Unreleased]: https://github.com/niravraychura/terradrift/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.2.0
[0.1.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.1.0
