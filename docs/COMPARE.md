# TerraDrift compared to other drift tools

TerraDrift is a **plan-based** CLI: it runs `terraform` or `tofu plan` on **your** runner (laptop, cron, GitHub Actions) and reports what that plan says changed. It does **not** crawl a cloud account for resources missing from state, apply changes, or host a control plane.

| You have | Use TerraDrift? |
| --- | --- |
| Scheduled `plan` on roots you already manage | Yes — that is the product |
| Want a list of unmanaged / unknown cloud resources | No — use inventory tools |
| Want apply, run queues, or a hosted UI | No — use an orchestrator |

## vs `terraform plan` / `tofu plan` alone

Terraform already detects drift. TerraDrift adds a stable report (JSON / table / JUnit / SARIF / Prometheus), exit codes for CI, redaction, policy as a publish gate, history, dashboards, and notifications. It does not replace the planner.

## vs driftctl, Cloud Concierge, and similar inventory scanners

Those tools enumerate cloud APIs and compare to state (or a similar inventory). They can find resources **Terraform does not manage**. TerraDrift never does that. If it is not in the plan for that root, we do not report it.

## vs HCP Terraform, Spacelift, env0, Scalr (orchestration)

Those products run plans/applies in their control plane, with run history, VCS checks, and often policy. TerraDrift is self-hosted CLI you invoke; you keep IAM, state backends, and runners. It can sit *beside* those systems (for example `--plan-file` on a plan you already trust) but it is not a workspace host.

## vs [rootsami/terradrift](https://github.com/rootsami/terradrift)

That project is a 2023 Go **server** (last push 2023) with a different architecture and GitHub path. This repository (`niravraychura/terradrift`) is a CLI. Same name, different product. TerraDrift is also not a HashiCorp or OpenTofu product — see the trademarks note in the README.

## Out of scope (still)

Auto-apply, remediating infrastructure, and a hosted SaaS — see [ROADMAP.md](ROADMAP.md).
