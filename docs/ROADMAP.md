# Roadmap

High-level product direction for TerraDrift. Detailed engineering work lives in [PLAN.md](../PLAN.md). Version buckets live as GitHub **Milestones** (`v0.1.0`, `v0.2.0`, `v1.0.0`). Triage uses a single GitHub **Project** board.

## Near term

| Milestone | Intent |
|-----------|--------|
| **v0.1.0** | First public tagged CLI release with documented install and release process |
| **v0.2.0** | Product polish after first release (UX, docs, scan-all / CI examples as needed) |
| **v1.0.0** | Stability promise for CLI flags and published JSON consumers rely on |

## Out of scope (for now)

- Hosted SaaS control plane
- Auto-apply / remediating infrastructure from TerraDrift
- Distributed or remote scan locks (local locks only)
- Replacing Terraform/OpenTofu as the planner

## Related

- [docs/RELEASE.md](RELEASE.md) — how versions ship
- [docs/GITHUB_PRODUCT_SETUP.md](GITHUB_PRODUCT_SETUP.md) — GitHub packaging checklist
- [SECURITY.md](../SECURITY.md) — trust boundary and reporting
