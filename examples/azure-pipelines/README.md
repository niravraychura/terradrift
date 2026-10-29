# Azure DevOps example

Copy [`azure-pipelines.yml`](azure-pipelines.yml) into your infrastructure repository. Same contract as the GitHub/GitLab scheduled examples: checksum-verifying `scripts/install.sh`, `--terraform-exec`, `--redact-paths`. Ubuntu agents only — TerraDrift does not publish Windows archives.

The official GitHub Action (`uses: niravraychura/terradrift@v1.1.1`) is GitHub-only. On Azure DevOps, pin `TERRADRIFT_VERSION` to a published GitHub Release tag (`vX.Y.Z`), not a commit SHA.

## Schedule

1. Merge the file as `azure-pipelines.yml` (or add it as a YAML pipeline).
2. `schedules:` runs on `main`. You can also add scheduled runs in the pipeline UI.
3. Keep webhook URLs and cloud credentials in secret pipeline variables, not in the YAML.

## Operator notes

- **OIDC:** uncomment `AzureCLI@2` and point `azureSubscription` at a Resource Manager service connection that uses a federated credential. Wire a Reader-scope identity as in [docs/DRIFT_SCAN_IAM.md](../../docs/DRIFT_SCAN_IAM.md). Do not store a client secret in the YAML.
- **Windows agents:** not supported. Use `vmImage: ubuntu-24.04` (or a Linux self-hosted pool).
- **Plans:** do not publish `*.tfplan` artifacts.
- **OpenTofu:** install `tofu` instead of Terraform and add `--terraform-bin tofu`.
