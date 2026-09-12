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
   - Open `checksums.txt` on the Release: lines must be bare names (`terradrift_linux_amd64.tar.gz`), not `dist/…`.
   - Spot-check: `TERRADRIFT_VERSION=vX.Y.Z PREFIX=$(mktemp -d) ./scripts/install.sh` succeeds for `linux_amd64` (or run `scripts/install_checksum_lines_test.sh`).
6. **Back-merge** — if `main` is ahead of `dev` (promotion merge commit), PR `main` → `dev` and merge after CI.
7. **Consume pins** — only after the GitHub Release archives exist. PR into `dev` (do not bump a default to a tag with no assets — `install.sh` will 404). Update:
   - `scripts/install.sh` default `TERRADRIFT_VERSION` and its usage comment
   - README install example and GHCR image tag
   - `examples/github-actions/*.yml` `TERRADRIFT_VERSION` (install.sh workflows)
   - Official Action examples / README / `docs/GITHUB_ACTION.md`: `uses: niravraychura/terradrift@vX.Y.Z` and drop `version:` (`action.yml` uses the `v*` ref). Until that tag includes `action.yml`, keep `uses: @dev` plus `version:` of the latest CLI release.
   - Homebrew tap: `./scripts/gen-homebrew-formula.sh vX.Y.Z` → commit `Formula/terradrift.rb` in `niravraychura/homebrew-tap` (see `contrib/homebrew/README.md`)
8. **Milestone** — if an open GitHub milestone exists for this version, **close** it after the release succeeds. Do not create empty milestones for future versions.
9. **v1.0.0 LinkedIn** — do **not** remind or post unless the maintainer explicitly asks. Copy lives on [#95](https://github.com/niravraychura/terradrift/issues/95) (parked from #111). Never post from the agent.

## Rules

- Never rewrite or move a published release tag.
- Pre-1.0: breaking CLI/JSON → call out in CHANGELOG; prefer MINOR bump.
- Hotfix on `main` → patch tag, then back-merge to `dev`.
- No dummy milestones (see `.cursor/rules/github-milestones.mdc`).
- Report PR URLs, tag, release URL, and CI/release conclusion in the final reply.
