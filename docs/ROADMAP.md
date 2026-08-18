# Roadmap

High-level product direction for TerraDrift. Detailed engineering work lives in [PLAN.md](../PLAN.md). Triage uses a single GitHub **Project** board.

**Milestones:** create a GitHub milestone only when there is real work to attach (issues/PRs). Do not keep empty version placeholders. Shipped releases close their milestone (see `.cursor/rules/github-milestones.mdc`).

## Near term (intent — not empty GitHub milestones)

| Version | Intent |
|---------|--------|
| **v0.1.0** | Shipped — first public tagged CLI release |
| **v0.2.0** | Product polish after first release (UX, docs, scan-all / CI examples as needed) — open a milestone when work starts |
| **v1.0.0** | Stability promise for CLI flags and published JSON consumers rely on — open a milestone when work starts |

## Out of scope (for now)

- Hosted SaaS control plane
- Auto-apply / remediating infrastructure from TerraDrift
- Distributed or remote scan locks (local locks only)
- Replacing Terraform/OpenTofu as the planner

## Related

- [docs/RELEASE.md](RELEASE.md) — how versions ship
- [docs/GITHUB_PRODUCT_SETUP.md](GITHUB_PRODUCT_SETUP.md) — GitHub packaging checklist
- [SECURITY.md](../SECURITY.md) — trust boundary and reporting
