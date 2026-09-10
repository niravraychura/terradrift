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
5. Confirm release binaries report the tag: `terradrift --version` (injected via `-ldflags` / Docker `VERSION` build-arg).
6. Back-merge `main` into `dev` if `main` gained commits (changelog, hotfixes) not already on `dev`.
7. After archives exist, PR consume-pin updates into `dev`: `scripts/install.sh` default, README install/GHCR examples, `examples/github-actions` `TERRADRIFT_VERSION`, and Action `uses:` (see [Install / consume](#install--consume)). Do not point those defaults at a tag that has no GitHub Release yet.

Do not force-push tags that have already been published with artifacts.

## Install / consume

- GitHub Releases: https://github.com/niravraychura/terradrift/releases
- Installer default: `scripts/install.sh` (`TERRADRIFT_VERSION`) must match the latest published tag. Bump it in a follow-up on `dev` after `release.yml` uploads archives — not in the tag commit itself if that would 404 during the gap before assets exist.
- Official Action: `uses: niravraychura/terradrift@vX.Y.Z` after a tag that includes `action.yml`. Empty `version:` then installs that tag. Until then, pin `uses:` to `dev` (or a SHA) and set `version:` to the latest CLI release.
- Container: GHCR image published by the release workflow (see Release notes for digest/tag). Keep the README `FROM` pin on the latest tag too.

## Related

- [SECURITY.md](../SECURITY.md) — vulnerability reporting and supported versions
- [CONTRIBUTING.md](../CONTRIBUTING.md) — branch and PR expectations
- [docs/GITHUB_PRODUCT_SETUP.md](GITHUB_PRODUCT_SETUP.md) — packaging summary (historical checklist archived)
