# Roadmap

High-level product direction for TerraDrift. Triage uses a single GitHub **Project** board.

**Milestones:** create a GitHub milestone only when there is real work to attach (issues/PRs). Do not keep empty version placeholders. Shipped releases close their milestone (see `.cursor/rules/github-milestones.mdc`).

Completed engineering history (through v0.1.0): [archive/PLAN-v0.1.md](archive/PLAN-v0.1.md) — see also the root [PLAN.md](../PLAN.md) stub.

## Near term (intent — not empty GitHub milestones)

| Version | Intent |
|---------|--------|
| **v0.1.0** | Shipped — first public tagged CLI release |
| **v0.2.0** | Product polish after first release — [milestone](https://github.com/niravraychura/terradrift/milestone/4) (version/DX, scan-all parity, ignore/baselines, GitHub PR upsert, install path) |
| **v1.0.0** | Stability promise for CLI flags and published JSON consumers rely on — open a milestone when work starts |

## Out of scope (for now)

- Hosted SaaS control plane
- Auto-apply / remediating infrastructure from TerraDrift
- Distributed or remote scan locks (local locks only)
- Replacing Terraform/OpenTofu as the planner

## Related

- [docs/RELEASE.md](RELEASE.md) — how versions ship
- [docs/GITHUB_PRODUCT_SETUP.md](GITHUB_PRODUCT_SETUP.md) — packaging summary (checklist archived)
- [SECURITY.md](../SECURITY.md) — trust boundary and reporting
