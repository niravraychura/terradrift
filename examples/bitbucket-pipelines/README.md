# Bitbucket Pipelines example

Copy [`bitbucket-pipelines.yml`](bitbucket-pipelines.yml) into your infrastructure repository. Same contract as the GitHub/GitLab scheduled examples: checksum-verifying `scripts/install.sh`, `--terraform-exec`, `--redact-paths`. Linux image only — TerraDrift does not publish Windows archives.

The official GitHub Action (`uses: niravraychura/terradrift@v1.1.1`) is GitHub-only. On Bitbucket, pin `TERRADRIFT_VERSION` to a published GitHub Release tag (`vX.Y.Z`), not a commit SHA.

## Schedule

1. Merge the file as `bitbucket-pipelines.yml`.
2. In Bitbucket: **Repository settings → Pipelines → Schedules** → custom pipeline `terradrift-scan`.
3. Keep webhook URLs and cloud credentials in repository/workspace variables, not in the YAML.

## Operator notes

- **OIDC:** the step sets `oidc: true` so `BITBUCKET_STEP_OIDC_TOKEN` is available. Wire AWS/GCP/Azure as in [docs/DRIFT_SCAN_IAM.md](../../docs/DRIFT_SCAN_IAM.md). Do not commit keys.
- **Plans:** do not publish `*.tfplan` artifacts.
- **OpenTofu:** install `tofu` instead of Terraform and add `--terraform-bin tofu`.
