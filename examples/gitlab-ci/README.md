# GitLab CI example

Copy [`.gitlab-ci.yml`](.gitlab-ci.yml) into your infrastructure repository. This is the same job as the GitHub scheduled example: checksum-verifying `scripts/install.sh`, `--terraform-exec`, `--redact-paths`. There is no GitLab-only binary or SaaS.

The official GitHub Action (`uses: niravraychura/terradrift@v1.1.1`) is GitHub-only. On GitLab, pin `TERRADRIFT_VERSION` to a published GitHub Release tag (`vX.Y.Z`), not a commit SHA.

## Schedule

1. Merge the file as `.gitlab-ci.yml` (or `include:` it).
2. In GitLab: **CI/CD → Schedules** → new schedule (cron). The job’s `rules` also allow a manual **Run pipeline** (`web`).
3. Keep webhook URLs and cloud credentials in CI/CD variables, not in the YAML.

## Operator notes

- **OIDC:** the job requests `GITLAB_OIDC_TOKEN`. Wire AWS/GCP/Azure as in [docs/DRIFT_SCAN_IAM.md](../../docs/DRIFT_SCAN_IAM.md). Self-managed GitLab: set `id_tokens.GITLAB_OIDC_TOKEN.aud` to your GitLab URL (`$CI_SERVER_URL`). Do not use long-lived keys or the removed `CI_JOB_JWT` variables.
- **Plugin cache:** `TF_PLUGIN_CACHE_DIR` plus GitLab `cache:` on `.terraform-plugin-cache/`.
- **Plans:** do not upload `*.tfplan` artifacts. Encrypted OpenTofu/Terraform state needs decrypt rights in CI; still do not publish the plan file.
- **OpenTofu:** install `tofu` instead of Terraform and add `--terraform-bin tofu`.
- **Terragrunt:** install `terragrunt` and scan stacked roots (`terragrunt.hcl`); `--terragrunt-bin` defaults to `terragrunt`.
