# Release policy

TerraDrift uses SemVer (`vMAJOR.MINOR.PATCH`) and ships artifacts from annotated tags on `main`.

## Branch flow

| Track | Rule |
|--------|------|
| Integration | All feature work merges to `dev` via pull request |
| Stable | Promote `dev` → `main` only when CI is green |
| Version | Annotated tag `vMAJOR.MINOR.PATCH` on `main` |
| Artifacts | Tag push runs `.github/workflows/release.yml` (archives, checksums, SBOM, GHCR image) |
| Pre-1.0 | Breaking CLI/JSON changes allowed in minors if called out in release notes |
| Hotfix | Patch on `main`, then back-merge to `dev` |
| Support | Security fixes on the latest tagged release / `main` until 1.0 |

**Cadence (pre-1.0):** release when ready after a coherent `dev` → `main` promotion—not on a forced calendar.

## Versioning

- **MAJOR** — breaking CLI flags, exit codes, or published JSON schema consumers rely on (after 1.0; before 1.0 prefer calling out breaks in notes and bumping MINOR)
- **MINOR** — new features, compatible flag additions
- **PATCH** — bug fixes, dependency bumps, docs-only that ship with a tag

Update [CHANGELOG.md](../CHANGELOG.md) before tagging.

## Cut a release

1. Ensure `dev` CI is green and promote to `main` (PR `dev` → `main` or fast-forward merge).
2. On `main`, update `CHANGELOG.md` (move Unreleased → version section with date).
3. Commit if needed, then create an annotated tag and push it:

```bash
git checkout main
git pull origin main
git tag -a v0.1.0 -m "TerraDrift v0.1.0"
git push origin v0.1.0
```

4. Confirm the GitHub Release and artifacts from `release.yml`.
5. Back-merge `main` into `dev` if `main` gained commits (changelog, hotfixes) not already on `dev`.

Do not force-push tags that have already been published with artifacts.

## Install / consume

- GitHub Releases: https://github.com/niravraychura/terradrift/releases
- Container: GHCR image published by the release workflow (see Release notes for digest/tag)

## Related

- [SECURITY.md](../SECURITY.md) — vulnerability reporting and supported versions
- [CONTRIBUTING.md](../CONTRIBUTING.md) — branch and PR expectations
- [docs/GITHUB_PRODUCT_SETUP.md](GITHUB_PRODUCT_SETUP.md) — packaging summary (historical checklist archived)
