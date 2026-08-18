# GitHub product packaging

**Status: complete** for TerraDrift’s public GitHub surface (as of v0.1.0).

## What this repo uses

| Area | Where |
|------|--------|
| Branch / release | [RELEASE.md](RELEASE.md) — `dev` → `main` → `v*` tag |
| Roadmap intent | [ROADMAP.md](ROADMAP.md) |
| Security reporting | [SECURITY.md](../SECURITY.md) |
| Contributing | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Agent / Cursor defaults | [AGENTS.md](../AGENTS.md), [`.cursor/`](../.cursor/) |
| One-shot Settings helper | [scripts/github-product-setup.sh](../scripts/github-product-setup.sh) |
| Project board | [TerraDrift Project](https://github.com/users/niravraychura/projects/1) |
| Historical checklist | [archive/GITHUB_PRODUCT_SETUP-v0.1.md](archive/GITHUB_PRODUCT_SETUP-v0.1.md) |

## Maintainer notes

- **Milestones:** create only when attaching real issues/PRs; close when the version ships (see `.cursor/rules/github-milestones.mdc`).
- **Optional later:** second-maintainer CODEOWNERS reviews; Cursor hooks.
- Re-run `scripts/github-product-setup.sh` after `gh auth login` if labels/topics/protection need repairing on a fork.
