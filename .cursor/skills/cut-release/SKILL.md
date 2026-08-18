---
name: cut-release
description: >-
  Cut a TerraDrift SemVer release (sync branches, promote dev to main, CHANGELOG,
  annotated v* tag, verify release.yml, back-merge). Use when the user asks to
  release, cut a version, tag vX.Y.Z, ship, or publish GitHub Release artifacts.
---

# Cut a TerraDrift release

Follow `docs/RELEASE.md`. Do not invent another flow. Get explicit version (e.g. `v0.2.0`) if missing.

## Steps

1. **Sync** — if `main` has commits not in `dev` (Dependabot, hotfixes), PR/merge `main` → `dev` first; wait for CI green.
2. **CHANGELOG** — on the release train (`dev`), move `[Unreleased]` notes into `[X.Y.Z] - YYYY-MM-DD`; keep an empty Unreleased section; PR into `dev`, CI green, merge.
3. **Promote** — open PR `dev` → `main`; wait for required `test` check; merge (no force-push).
4. **Tag on `main`** — annotated tag only:

```bash
git fetch origin && git checkout main && git pull origin main
git tag -a vX.Y.Z -m "TerraDrift vX.Y.Z"
git push origin vX.Y.Z
```

5. **Verify** — watch `.github/workflows/release.yml` for that tag; confirm GitHub Release assets (archives, checksums, SBOM) and GHCR image.
6. **Back-merge** — if `main` is ahead of `dev` (promotion merge commit), PR `main` → `dev` and merge after CI.

## Rules

- Never rewrite or move a published release tag.
- Pre-1.0: breaking CLI/JSON → call out in CHANGELOG; prefer MINOR bump.
- Hotfix on `main` → patch tag, then back-merge to `dev`.
- Report PR URLs, tag, release URL, and CI/release conclusion in the final reply.
