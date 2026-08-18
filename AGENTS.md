# AGENTS.md

Default brief for coding agents (Cursor, Cloud Agents, and similar) working in this repository.

## Product

TerraDrift is an **open-source, self-hosted** Terraform/OpenTofu **drift detection CLI** (Go). MIT licensed. Trusted-runner model — see `SECURITY.md`.

## Always-on agent behavior

1. **Ponytail** (`.cursor/rules/ponytail.mdc`) — YAGNI, reuse, stdlib-first, smallest correct diff.
2. **Engineering quality** — readability, maintainability, tests, performance bounds, security fail-closed.
3. **Ask before assuming** — clarify ambiguous requirements; if the user has no further instructions, recommend a default and proceed.
4. **Verify** — run tests before push; after push, confirm GitHub PR/CI is green.

## Branch and PR defaults

- Branch from **`dev`**; open PRs into **`dev`**.
- Promote **`dev` → `main`** only for stable ship; **never** invent a different release flow.
- Releases: annotated **`v*`** tags on **`main`** per `docs/RELEASE.md`. Tag push runs `release.yml`.
- Forward intent: `docs/ROADMAP.md`. Shipping: `docs/RELEASE.md`. Packaging summary: `docs/GITHUB_PRODUCT_SETUP.md`.
- Historical engineering ledger: `docs/archive/PLAN-v0.1.md` (root `PLAN.md` is a stub).

## Commands

```bash
make build
make test
make ci    # full local gate when tooling is available
```

## Safety

- Do **not** commit credentials, state, plans, webhook URLs, or realistic secret-like fixture strings.
- Prefer synthetic placeholders (`REDACTED_TEST_VALUE`, `https://example.test/...`).
- Do not weaken fail-closed I/O, policy-as-publish-gate, or paths-only attribute defaults without an explicit product decision and docs updates.
- Security reports belong in private advisories (`SECURITY.md`), not public issues.

## Source of truth

| Topic | File |
|--------|------|
| Contribute / PR expectations | `CONTRIBUTING.md` |
| Release / SemVer | `docs/RELEASE.md` |
| Roadmap | `docs/ROADMAP.md` |
| Security posture | `SECURITY.md` |
| Architecture | `docs/ARCHITECTURE.md` |
| Cursor rules | `.cursor/rules/` |
| Project skills | `.cursor/skills/cut-release`, `.cursor/skills/add-scan-flag` |
| Historical plans | `docs/archive/` (root `PLAN.md` is a stub) |

Do not contradict `docs/RELEASE.md` or `CONTRIBUTING.md` when proposing git workflow.
