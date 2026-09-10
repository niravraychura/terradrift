# Drift Scan IAM

Run TerraDrift with separate, read-only credentials per environment. Refresh-only planning still calls provider APIs and may read resource metadata, so restrict the credential to the Terraform root being scanned.

## GitHub Actions OIDC

Prefer workload identity (OIDC) over long-lived keys or PATs. Example jobs set `permissions.id-token: write` and `contents: read`. The official Action uses `GITHUB_TOKEN` from the job for `--github-pr` / `--github-issue-after`; do not add a PAT for that path.

AWS:

```yaml
- uses: aws-actions/configure-aws-credentials@cbe3b392738ccf3f987d68400dafcf4b0624a56c # v6.2.4
  with:
    role-to-assume: arn:aws:iam::ACCOUNT_ID:role/terradrift-readonly
    aws-region: us-east-1
```

GCP: `google-github-actions/auth@7c6bc770dae815cd3e89ee6cdf493a5fab2cc093` (v3) with Workload Identity Federation. Azure: `azure/login@a641126d1b8aa4d1fa005f4f92df94a3a4c4c906` (v3.1.0) with a federated credential on a Reader identity.

Pin third-party actions to SHAs in real workflows. Full scheduled examples: [examples/github-actions](../examples/github-actions/README.md).

## AWS

Use a dedicated IAM role with only the service `Describe*`, `Get*`, and `List*` permissions needed by the providers in the scanned root. Do not grant write actions, IAM administration, or unrestricted `sts:AssumeRole`. Scope resources and regions where the provider supports it.

## Azure

Use a dedicated service principal or managed identity with the Reader role at the smallest practical resource group or subscription scope. Add provider-specific read permissions only when Reader is insufficient. Do not assign Contributor or Owner.

## GCP

Use a dedicated service account with viewer-style roles narrowed to the required projects and services. Avoid Editor, Owner, and Service Account Token Creator. Use workload identity federation in CI instead of downloadable keys.

## Secret Scanning

Enable GitHub secret scanning and push protection where available. Run gitleaks or an equivalent scanner locally and in CI before publishing releases. Never commit state files, plans, `.terraform` directories, webhook URLs, tokens, artifact URLs, or cloud credentials.
