# GitHub Actions examples

Copy these into your infrastructure repository. Pin third-party actions to SHAs. Keep cloud credentials and webhook URLs in GitHub secrets.

## Official Action (preferred)

[`terradrift-action.yml`](terradrift-action.yml) uses the composite Action in this repo. It always passes `--terraform-exec` and fails if Terraform/OpenTofu is missing.

Until the first release that contains `action.yml` (v0.4.0+), set `version:` to a published CLI tag (`v0.3.0`) and pin `uses:` to `dev` or a commit SHA. After that tag: `uses: niravraychura/terradrift@v0.4.0`.

Marketplace listing is a GitHub UI step after that tagged release — see [docs/GITHUB_ACTION.md](../../docs/GITHUB_ACTION.md).

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
- **GitHub comments/issues:** add `pull-requests: write` for `--github-pr`, `issues: write` for `--github-issue-after`.
- **Plans:** do not upload `*.tfplan` artifacts. Encrypted OpenTofu/Terraform state needs decrypt rights in CI; still do not publish the plan file.
