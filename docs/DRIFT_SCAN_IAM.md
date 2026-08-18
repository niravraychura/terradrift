# Drift Scan IAM

Run TerraDrift with separate, read-only credentials per environment. Refresh-only planning still calls provider APIs and may read resource metadata, so restrict the credential to the Terraform root being scanned.

## AWS

Use a dedicated IAM role with only the service `Describe*`, `Get*`, and `List*` permissions needed by the providers in the scanned root. Do not grant write actions, IAM administration, or unrestricted `sts:AssumeRole`. Scope resources and regions where the provider supports it.

## Azure

Use a dedicated service principal or managed identity with the Reader role at the smallest practical resource group or subscription scope. Add provider-specific read permissions only when Reader is insufficient. Do not assign Contributor or Owner.

## GCP

Use a dedicated service account with viewer-style roles narrowed to the required projects and services. Avoid Editor, Owner, and Service Account Token Creator. Use workload identity federation in CI instead of downloadable keys.

## Secret Scanning

Enable GitHub secret scanning and push protection where available. Run gitleaks or an equivalent scanner locally and in CI before publishing releases. Never commit state files, plans, `.terraform` directories, webhook URLs, tokens, artifact URLs, or cloud credentials.
