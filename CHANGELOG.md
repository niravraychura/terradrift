# Changelog

All notable changes to TerraDrift are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- GitHub product packaging: release policy, contributing guide, issue/PR templates, Cursor/agent defaults, and roadmap docs.

## [0.1.0] - TBD

First public tagged release of the self-hosted TerraDrift CLI.

### Added

- `scan` and `scan-all` with Terraform/OpenTofu execution (`--terraform-exec`), refresh-only and normal plan modes
- Workspace selection and `-var` / `-var-file` passthrough
- Table and JSON reports with attribute-level diffs (paths-only by default; secrets redacted)
- Policy publish gate, cost/audit adapters, Slack/Teams/webhook/GitHub notifications
- History, static dashboards, JUnit/SARIF/Prometheus outputs, severity gates
- Multi-root manifests with concurrent scans and shared delivery options
- CI, Dependabot, and release workflow (archives, checksums, SBOM, signed GHCR image)

### Security

- Trusted-runner model documented in `SECURITY.md`
- SSRF-safe GitHub HTTP client, fail-closed truncated I/O, attribute value heuristics
- Size budgets and redaction defaults for user-facing output

[Unreleased]: https://github.com/niravraychura/terradrift/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/niravraychura/terradrift/releases/tag/v0.1.0
