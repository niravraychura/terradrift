# Compatibility (v1.0)

This is the SemVer contract for CLI flags, process exit codes, and published JSON. It bound at the **`v1.0.0` tag** on `main`. Removing or renaming a contracted flag, exit code, or JSON field is a **MAJOR**.

## Commands

These command names are part of the contract:

- `terradrift scan`
- `terradrift scan-all`
- `terradrift approve`
- `terradrift init`
- `terradrift serve`
- `terradrift dashboard-index`

Removing a command, or changing what it is for, is a **MAJOR**. Adding a command is a **MINOR**.

`terradrift --help` / `<command> --help` at the 1.0 tag is the flag list. Removing a flag, renaming it, or changing its meaning is a **MAJOR**. Adding a flag or an optional config field is a **MINOR**.

`--approval-file` stays review-only and does not suppress exit 2. Ignores/baselines remain the CI pass path.

## Exit codes

| Code | Meaning |
|------|---------|
| **0** | Success, including `no_drift`, `no_changes`, and `skipped` (for example `--skip-if-open-pr`) |
| **1** | Failure: usage, I/O, Terraform/adapter errors, policy/publish-gate failure, `scan-all` with failed roots |
| **2** | Findings: `drift_detected` or `changes_detected` after ignore/baseline rules |

`--failure-severity` may keep exit 0 when remaining drift is below the threshold; that is existing behavior, not a silent pass for all drift.

Stdout can print a report **before** the policy gate. Treat the process exit code as the gate, not the JSON body alone.

## Published JSON

### `scan` (`--output json`)

Stable fields (always present unless noted): `scan_id`, `status`, `directory`, `plan_mode`, `total_resources_checked`, `resources_checked_exact`, `total_changed_resources`, `resource_changes`, `started_at`, `completed_at`. Optional: `root_id`.

`status` values: `no_drift`, `drift_detected`, `no_changes`, `changes_detected`, `failed`, `skipped`. `running` is internal and is not a completed stdout report.

`--plan-mode both` is additive. The report keeps a single `status` (`drift_detected` if refresh-only found anything, otherwise `changes_detected` if the normal plan did). Optional fields: `config_status`, and `change_kind` (`refresh` or `config`) on `resource_changes[]`. `--plan-file` cannot be combined with `both`.

New **optional** fields may be added (**MINOR**). Removing/renaming a stable field or changing a `status` string is a **MAJOR**. Consumers must ignore unknown fields.

Attribute `before`/`after` may be omitted (`omitempty`) in paths-only reports. Secrets stay redacted. See [ARCHITECTURE.md](ARCHITECTURE.md).

### `scan-all` (`--output json`)

Stable aggregate fields: `status`, `roots`, `total_roots`, `drifted_roots`, `changed_roots`, `failed_roots`, `total_resources_checked`, `total_changed_resources`.

Aggregate `status` values: `complete`, `drift_detected`, `changes_detected`, `partial`, `failed`, `skipped`. Each `roots[]` entry has `directory` and may include `report` (a scan report) or `error`.

### Not a compatibility API

These may change without a MAJOR: table/text output, help and log lines, dashboard HTML, SARIF, JUnit, Prometheus wording, Slack/Teams/GitHub comment bodies, `--notify webhook` payload wording, on-disk history wrapping, incremental `scan-all` state files, and `internal/` Go APIs.

`.terradrift.json` may gain keys (**MINOR**). Removing or renaming a config key that `terradrift.schema.json` advertised at 1.0 is a **MAJOR**.

## After 1.0

- **MAJOR** — breaking flags, exit codes, or published JSON above
- **MINOR** — compatible additions
- **PATCH** — fixes and docs that ship with a tag

Out of scope stays: hosted control plane, auto-apply, unmanaged cloud inventory, replacing Terraform/OpenTofu as the planner ([ROADMAP.md](ROADMAP.md)).
