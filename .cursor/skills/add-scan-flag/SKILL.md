---
name: add-scan-flag
description: >-
  Add or change a TerraDrift scan/scan-all CLI flag end-to-end (cmd wiring, config
  schema, tests, README). Use when adding a flag, CLI option, config field for
  scan settings, or changing flag behavior/exit codes.
---

# Add a scan flag

Ponytail first: confirm the flag is needed; reuse existing flags/config if enough.

## Touch points (minimal set)

1. **`cmd/terradrift/scan.go`** — declare flag; wire into scan options / overlays (see existing `Flags()` and config overlay patterns).
2. **`cmd/terradrift/scan_all.go`** — only if multi-root should share the flag; keep `scan-all` delivery subset honest in README if not supported.
3. **`internal/config`** + **`docs/terradrift.schema.json`** — if the setting is config/profile-backed; CLI wins over config.
4. **Tests** — table-driven unit tests for parsing/defaults; command tests in `cmd/terradrift/*_test.go` when flag affects CLI behavior. Synthetic fixtures only (`fixtures-no-secrets`).
5. **Docs** — update README “Common options” or Quick start example (not a flag dump); ensure `--help` text is clear (`docs-with-flags` rule).
6. **Examples** — update `examples/config` only if operators need a sample.

## Conventions

- Fail closed at trust boundaries; do not weaken paths-only attributes, redaction, or policy-as-publish-gate without product + docs.
- Prefer stdlib and existing `internal/` helpers.
- After code: `go test` for touched packages (prefer `make test`); PR into **`dev`**; watch CI (`verify-before-done`).

## Done when

- Flag works on intended command(s), tests pass, schema/README/`--help` aligned, PR CI green.
