# TerraDrift

Self-hosted Terraform/OpenTofu **drift detection CLI** (Go, MIT). Trusted-runner model. Single binary: `./cmd/terradrift`.

## Pipeline (do not reorder)

`cmd/terradrift` → config → scanner → terraform → parser → report → stdout → **policy publish gate** → history/dashboard/artifacts/notify.

Policy runs **after stdout**. Gate failure skips side effects; process still exits non-zero. CI that only greps stdout can miss a policy fail.

## Layout

- `cmd/terradrift` — Cobra: `scan`, `scan-all`, `init`, `approve`, `serve`/`dashboard-index`, delivery. Exit helpers: `exitCodeOK`/`Failure`/`DriftDetected`.
- `internal/config` — YAML/JSON schema; CLI overlays win.
- `internal/scanner` — roots, TF-backed locks, outcomes.
- `internal/terraform` — bounded Terraform/OpenTofu exec.
- `internal/parser` — plan JSON → deterministic diffs.
- `internal/report` — risk/ownership/remediation; `WithoutAttributeValues` for persist defaults.
- `internal/policy` — publish gate.
- `internal/notify`, `dashboard`, `history`, `redact`, `command` — delivery, persistence, redaction, adapter exec.

## Invariants

- Fail closed at adapters, webhooks, policy I/O, SSRF, size/timeout budgets.
- Default persist/automation: **attribute paths only** unless `--attribute-values`. Secrets stay redacted even then.
- `--terraform-exec` required for real scans; GitHub Actions / `TERRADRIFT_REQUIRE_EXEC` fail if omitted.
- No credentials, TF state, plans, webhook URLs, or realistic secret fixtures in-repo. Placeholders: `REDACTED_TEST_VALUE`, `https://example.test/...`.
- Security reports → private advisory (`SECURITY.md`), never public exploit issues.
- Branch from `dev`; PR into `dev`. Releases: `dev`→`main` then annotated `v*` on `main` (`docs/RELEASE.md`).
- Official Action: `uses: @vX.Y.Z` needs no `version:`. SHA/branch pin **must** set `with.version: vX.Y.Z`.

## Agent graph

- Languages/build: `mem:tech_stack`
- Commands: `mem:suggested_commands`
- Style / fail-closed / tracker: `mem:conventions`
- Done-gate: `mem:task_completion`
- Memory style: `mem:memory_maintenance`
