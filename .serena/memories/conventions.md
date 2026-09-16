# Conventions

Human + agent source of truth: root `AGENTS.md`, `.cursor/rules/` (Ponytail, tracker-first, verify-before-done, go-cli, fixtures-no-secrets, agent-skills).

- Smallest correct diff; stdlib/`internal/` before new deps; no drive-by refactors.
- Tracker-first: search GitHub+Linear (project **TerraDrift**), file if missing, PR into `dev` with `Closes #N` + Linear ID. Skip tickets only if user says so, or security (private advisory).
- Ask before assuming; if user cannot decide, pick a default and say so.
- Tests: table-driven; synthetic fixtures only. New/changed Go behavior: failing test first (`mem:task_completion`). Trivial docs/one-liners: no test.
- Comments: why (invariants, trust boundaries), not what.
- Do not weaken paths-only attributes, redaction, or policy-as-publish-gate without an explicit product decision + docs.
- Flag/config changes: README + `--help` (`docs-with-flags`); schema `docs/terradrift.schema.json` when config-backed.
- `gofmt`; wrap errors like surrounding packages.
- Skills auto-invoke: `.cursor/rules/agent-skills.mdc`. Product runbooks: `.cursor/skills/`. Third-party: `.agents/skills/`.
