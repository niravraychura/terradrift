# GitHub Actions examples

Copy these into your infrastructure repository. Pin third-party actions to SHAs. Keep cloud credentials and webhook URLs in GitHub secrets.

## Official Action (preferred)

[`terradrift-action.yml`](terradrift-action.yml) uses the composite Action in this repo. It always passes `--terraform-exec` and fails if Terraform/OpenTofu is missing.

Pin `uses: niravraychura/terradrift@v0.4.1`. Empty `version:` installs that tag. `action.yml` has shipped since v0.4.0.

Marketplace: [TerraDrift Scan](https://github.com/marketplace/actions/terradrift-scan).

## Copy-paste workflows

These install TerraDrift with checksum-verifying `scripts/install.sh` (not `v0.0.0`).

| File | Use |
|------|-----|
| [`terradrift-scheduled.yml`](terradrift-scheduled.yml) | Cron + Slack |
| [`terradrift-pr.yml`](terradrift-pr.yml) | PR comment upsert (`GITHUB_TOKEN`) |
| [`terradrift-opentofu.yml`](terradrift-opentofu.yml) | OpenTofu (`--terraform-bin tofu`) |
| [`terradrift-scheduled-multi-root.yml`](terradrift-scheduled-multi-root.yml) | `scan-all` + severity gate |

## Operator notes

- **OIDC:** jobs set `id-token: write`. Wire AWS/GCP/Azure as in [docs/DRIFT_SCAN_IAM.md](../../docs/DRIFT_SCAN_IAM.md). Do not use a long-lived PAT for cloud auth.
- **Plugin cache:** `TF_PLUGIN_CACHE_DIR` plus `actions/cache` keyed on `.terraform.lock.hcl`.
- **GitHub comments/issues:** add `pull-requests: write` for `--github-pr`, `issues: write` for `--github-issue-after` (one issue per root+fingerprint; closed when that root scans clean). Optional `--github-issue-label` (at most 8). GHES/GHEC: Actions sets `GITHUB_API_URL`; TerraDrift uses it (HTTPS, no userinfo). `--skip-if-open-pr` needs `pull-requests: read` and `--github-repository`; it skips when an open PR changes files under that root.
- **Terragrunt:** scan a stacked root with `terraform-bin: terragrunt` (or `args: --terragrunt-bin /path/to/terragrunt`). Install Terragrunt in the job; Terraform/OpenTofu remains the planner.
- **Plans:** do not upload `*.tfplan` artifacts. Encrypted OpenTofu/Terraform state needs decrypt rights in CI; still do not publish the plan file.
- **Exit 2:** `--approval-file` is audit-only; known drift still fails the job. Use `ignore_rules` / `baseline_rules` to pass CI. PagerDuty/Opsgenie: map `--notify webhook` in an adapter ([examples/webhooks](../webhooks)).
