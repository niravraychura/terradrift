# Roadmap

High-level product direction for TerraDrift. Triage uses a single GitHub **Project** board.

**Milestones:** create a GitHub milestone only when there is real work to attach (issues/PRs). Do not keep empty version placeholders. Shipped releases close their milestone (see `.cursor/rules/github-milestones.mdc`).

Completed engineering history (through v0.1.0): [archive/PLAN-v0.1.md](archive/PLAN-v0.1.md) — see also the root [PLAN.md](../PLAN.md) stub.

## Near term (intent — not empty GitHub milestones)

| Version | Intent |
|---------|--------|
| **v0.1.0** | Shipped — first public tagged CLI release |
| **v0.2.0** | Shipped — `--version`, scan-all delivery parity, refresh-only attribute diffs |
| **v0.3.0** | Shipped — operator UX, glob ignores, GitHub PR upsert, install.sh / extra GOARCH |
| **v0.4.0** | Shipped — CI honesty, lock/plan honesty, GitHub Action, `--plan-file`, CLI Cosign |
| **v1.0.0** | Stability promise for CLI flags and published JSON — [#95](https://github.com/niravraychura/terradrift/issues/95); open a milestone when freeze work starts |

Triage: [TerraDrift Project](https://github.com/users/niravraychura/projects/1). **Ready** is empty. **Backlog** is later (v1.0 freeze).

## Out of scope (for now)

- Hosted SaaS control plane
- Auto-apply / remediating infrastructure from TerraDrift
- Distributed or remote scan locks (local locks only)
- Replacing Terraform/OpenTofu as the planner

## Related

- [docs/RELEASE.md](RELEASE.md) — how versions ship
- [docs/GITHUB_PRODUCT_SETUP.md](GITHUB_PRODUCT_SETUP.md) — packaging summary (checklist archived)
- [SECURITY.md](../SECURITY.md) — trust boundary and reporting
