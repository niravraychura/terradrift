# GitHub Product Setup

Actionable backlog for TerraDrift as a public GitHub product. This is **not** engineering feature work (`PLAN.md`); it is repo packaging, release process, community surface, and Cursor defaults.

**Status:** complete for GitHub packaging. First public tag `v0.1.0` is cut from `main` after promote (see Phase 1.4). Optional stale-branch cleanup remains.

**Project board:** https://github.com/users/niravraychura/projects/1 (linked to `terradrift`; Status: Backlog → Ready → In progress → In review → Done)

**Already in place before this work:**
- MIT `LICENSE`
- `SECURITY.md`
- `dev` / `main` branch model
- Dependabot (`gomod`, `github-actions`, `docker`)
- CI (`.github/workflows/ci.yml`) and release (`.github/workflows/release.yml`)
- README + `docs/` architecture and adapter docs

---

## Goals

1. Anyone landing on the repo understands how to report bugs, contribute, and consume releases.
2. Releases are predictable: `dev` → `main` → annotated `v*` tag → GitHub Release artifacts.
3. Cursor (and other agents) inherit the same defaults as humans via committed rules and templates.

---

## Phase 1 — Release cycle and branch policy

### 1.1 Release cycle policy document

- [x] Add `docs/RELEASE.md` with the policy below (keep it short; link from README).
- [x] Link `docs/RELEASE.md` from README (“Releasing”) and from `SECURITY.md` if support window is stated there.

**Policy encoded in `docs/RELEASE.md`:**

| Track | Rule |
|--------|------|
| Integration | All feature work merges to `dev` via PR |
| Stable | Promote `dev` → `main` only when CI is green |
| Version | Annotated tag `vMAJOR.MINOR.PATCH` on `main` |
| Artifacts | Tag push runs `release.yml` (archives, checksums, SBOM, GHCR image) |
| Pre-1.0 | Breaking CLI/JSON changes allowed in minors if called out in release notes |
| Hotfix | Patch on `main`, then back-merge to `dev` |
| Support | Security fixes on latest tagged release / `main` until 1.0 |

### 1.2 Branch protection (GitHub Settings → Branches)

- [x] Protect `main`: require PR, require status checks (`test`), no force-push.
- [x] Protect `dev`: require PR, require status checks (`test`), no force-push.
- [ ] Optional: require 1 approving review + CODEOWNERS once there is a second maintainer.

### 1.3 Dependabot targets `dev`

- [x] Update `.github/dependabot.yml` so all ecosystems use `target-branch: dev`.
- [x] No open Dependabot PRs currently targeting `main`.

### 1.4 First public release

- [x] Sync `main` → `dev`, then promote `dev` → `main` for the ship set.
- [x] `CHANGELOG.md` dated for `0.1.0` (2026-08-19).
- [x] Annotated tag `v0.1.0` on `main` (see `docs/RELEASE.md`).
- [ ] Verify GitHub Release + artifacts from `release.yml` after tag push.
- [x] README badges + Releases link (badge populates after first tag).

---

## Phase 2 — Issues, labels, milestones, Project

### 2.1–2.2 Templates

- [x] Issue templates + PR template

### 2.3 Labels

- [x] Labels created (`bug`, `enhancement`, `security`, `docs`, `good first issue`, `dependencies`, `release`, …)

### 2.4 Milestones

- [x] Policy: **no empty/dummy milestones** (`.cursor/rules/github-milestones.mdc`)
- [x] `v0.1.0` closed after ship; empty `v0.2.0` / `v1.0.0` placeholders removed
- [x] Version intent documented in `docs/ROADMAP.md`; create a milestone only when attaching real issues/PRs

### 2.5 Single GitHub Project

- [x] Project **TerraDrift** created and linked to repo
- [x] Status options: **Backlog → Ready → In progress → In review → Done**
- [x] Use Project for triage; `PLAN.md` remains engineering detail

### 2.6 Discussions

- [x] Discussions enabled

### 2.7 Security reporting settings

- [x] Private vulnerability reporting enabled
- [x] `SECURITY.md` matches private advisory reporting path

---

## Phase 3 — Contributor and maintainer files

- [x] `CONTRIBUTING.md`
- [x] `.github/CODEOWNERS`
- [x] `docs/ROADMAP.md` + README links

---

## Phase 4 — Cursor and agent defaults

- [x] `.cursor/rules/terradrift-workflow.mdc` (`alwaysApply: true`)
- [x] `.cursor/rules/go-cli.mdc`
- [x] Root `AGENTS.md`
- [ ] Optional hooks/skills — deferred
- [x] Rules committed (no secrets / machine paths)

---

## Phase 5 — Hygiene and cleanup

- [ ] Delete or archive stale remote branches that are fully merged (when convenient)
- [x] Repo description + topics (`terraform`, `drift`, `golang`, `cli`, `opentofu`, `infrastructure-as-code`)
- [x] Default branch `main`; development PRs target `dev`
- [x] README CI + release badges

**Do not overbuild yet:** multiple Projects, SLA dashboards, stale-bot spam, complex OKR boards.

---

## Next operator step — cut `v0.1.0`

```bash
# 1) Sync: open PR main→dev or merge main into a sync branch if needed
# 2) Promote: PR dev → main when CI is green
# 3) Tag on main:
git checkout main && git pull
# set CHANGELOG date for 0.1.0, commit
git tag -a v0.1.0 -m "TerraDrift v0.1.0"
git push origin v0.1.0
```

See `docs/RELEASE.md`.

---

## Source of truth map

| Concern | Document / place |
|---------|------------------|
| Engineering features & hardening | `PLAN.md` |
| GitHub product packaging (this backlog) | `docs/GITHUB_PRODUCT_SETUP.md` |
| How we release | `docs/RELEASE.md` |
| Security reporting & posture | `SECURITY.md` |
| How humans contribute | `CONTRIBUTING.md` |
| How Cursor/agents behave by default | `AGENTS.md` + `.cursor/rules/` |
| Triage board | [TerraDrift Project](https://github.com/users/niravraychura/projects/1) |
| Version buckets | GitHub Milestones (only when work is attached; see `.cursor/rules/github-milestones.mdc`) |
| Automate Settings | `scripts/github-product-setup.sh` |

---

## Out of scope for this document

- Changing MIT license
- Implementing product features listed in `PLAN.md`
- SaaS / paid offering setup
- Legal review of license or trademarks
