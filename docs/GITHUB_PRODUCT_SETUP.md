# GitHub Product Setup

Actionable backlog for TerraDrift as a public GitHub product. This is **not** engineering feature work (`PLAN.md`); it is repo packaging, release process, community surface, and Cursor defaults.

**Status:** in progress — repo files land via PR; GitHub Settings items via `scripts/github-product-setup.sh` (requires `gh auth login`).

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

- [ ] Protect `main`: require PR, require status checks (CI), no force-push, no direct commits.
- [ ] Protect `dev`: require PR, require status checks (CI), no force-push.
- [ ] Optional: require 1 approving review + CODEOWNERS once there is a second maintainer.

Run: `./scripts/github-product-setup.sh` (after `gh auth login`).

### 1.3 Dependabot targets `dev`

- [x] Update `.github/dependabot.yml` so all ecosystems use `target-branch: dev`.
- [ ] Confirm open Dependabot PRs are retargeted or closed/reopened against `dev` (script does best-effort retarget).

### 1.4 First public release

- [ ] Ensure `main` is current with the intended ship set (promote from `dev` if needed).
- [x] Add `CHANGELOG.md` (Keep a Changelog style); finalize date when tagging.
- [ ] Create annotated tag `v0.1.0` on `main` and push the tag (see `docs/RELEASE.md`).
- [ ] Verify GitHub Release + artifacts from `release.yml`.
- [x] README badges + Releases link (badge populates after first tag).

---

## Phase 2 — Issues, labels, milestones, Project

### 2.1 Issue templates

- [x] Add `.github/ISSUE_TEMPLATE/bug_report.yml`
- [x] Add `.github/ISSUE_TEMPLATE/feature_request.yml`
- [x] Add `.github/ISSUE_TEMPLATE/config.yml` with security contact link

### 2.2 Pull request template

- [x] Add `.github/pull_request_template.md`

### 2.3 Labels

| Label | Use |
|--------|-----|
| `bug` | Defect |
| `enhancement` | Feature / improvement |
| `security` | Security-sensitive |
| `docs` | Documentation only |
| `good first issue` | Newcomer-friendly |
| `dependencies` | Dependabot / deps |
| `release` | Release / versioning work |

- [ ] Labels created in GitHub (`./scripts/github-product-setup.sh`)

### 2.4 Milestones

| Milestone | Intent |
|-----------|--------|
| `v0.1.0` | First public tagged CLI release |
| `v0.2.0` | Next product slice after 0.1 |
| `v1.0.0` | CLI/JSON stability promise |

- [ ] Create milestones in GitHub (`./scripts/github-product-setup.sh`)
- [ ] Attach open issues/PRs as they appear; leave dates empty unless useful

### 2.5 Single GitHub Project

- [ ] Create one Project (board): **Backlog → Ready → In progress → In review → Done**
- [ ] Use Project for triage; keep `PLAN.md` as engineering detail
- [ ] Optional: add Project views filtered by milestone / label

```bash
gh project create --owner niravraychura --title "TerraDrift" --format json
```

### 2.6 Discussions

- [ ] Enable Discussions for Q&A / ideas (`./scripts/github-product-setup.sh` or Settings → General)

### 2.7 Security reporting settings

- [ ] Enable private vulnerability reporting (script or Settings → Code security)
- [x] `SECURITY.md` matches private advisory reporting path

---

## Phase 3 — Contributor and maintainer files

### 3.1 CONTRIBUTING.md

- [x] Root `CONTRIBUTING.md` (branch from `dev`, tests, no secrets, links)

### 3.2 CODEOWNERS

- [x] `.github/CODEOWNERS` for workflows, security, release docs, Dockerfile, Cursor rules

### 3.3 Roadmap

- [x] `docs/ROADMAP.md` + README links

---

## Phase 4 — Cursor and agent defaults

### 4.1 Project rules (`.cursor/rules/`)

- [x] `.cursor/rules/terradrift-workflow.mdc` (`alwaysApply: true`)
- [x] `.cursor/rules/go-cli.mdc` (Go globs)

### 4.2 AGENTS.md

- [x] Root `AGENTS.md`

### 4.3 Optional Cursor hooks / skills

- [ ] Deferred — only if needed after the files above are in daily use

### 4.4 Keep defaults in git

- [x] `.cursor/rules/*.mdc` and `AGENTS.md` committed with this change set
- [x] No secrets or personal machine paths in rules

---

## Phase 5 — Hygiene and cleanup

- [ ] Delete or archive stale remote branches that are fully merged (after verifying)
- [ ] Confirm repo About blurb + topics (`./scripts/github-product-setup.sh`)
- [x] Default branch remains `main` for consumers; development PRs still target `dev`
- [x] README CI + release badges

**Do not overbuild yet:** multiple Projects, SLA dashboards, stale-bot spam, complex OKR boards.

---

## Apply GitHub Settings (operator step)

After this PR is on `dev` (and preferably merged), with a valid `gh` session:

```bash
gh auth login -h github.com   # if token invalid
chmod +x scripts/github-product-setup.sh
./scripts/github-product-setup.sh
```

Then:

1. Create the Project board and columns (script prints the `gh project create` hint).
2. Promote `dev` → `main` when ready.
3. Cut `v0.1.0` per `docs/RELEASE.md` and set the date in `CHANGELOG.md`.

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
| Triage board | GitHub Project (one board) |
| Version buckets | GitHub Milestones |
| Automate Settings | `scripts/github-product-setup.sh` |

---

## Out of scope for this document

- Changing MIT license
- Implementing product features listed in `PLAN.md`
- SaaS / paid offering setup
- Legal review of license or trademarks
