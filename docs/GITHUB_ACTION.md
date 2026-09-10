# GitHub Action

TerraDrift ships a composite Action at the repository root (`action.yml`).

```yaml
- uses: hashicorp/setup-terraform@v4
  with:
    terraform_wrapper: false
- uses: niravraychura/terradrift@v0.4.0
  with:
    directory: ./terraform/prod
```

The Action always passes `--terraform-exec`, fails if `terraform` / `tofu` is missing, sets `--workspace-root` to `github.workspace`, and caches providers in `TF_PLUGIN_CACHE_DIR` unless `plugin-cache: false`.

Happy-path GitHub delivery uses `GITHUB_TOKEN` from the job (`--github-pr` / `--github-issue-after`). Do not put a PAT in the workflow. Cloud auth is OIDC — [DRIFT_SCAN_IAM.md](DRIFT_SCAN_IAM.md).

## Inputs

| Input | Default | Notes |
|-------|---------|--------|
| `version` | action `v*` ref | Required when `uses:` is not a release tag |
| `directory` | `.` | Terraform root |
| `terraform-bin` | `terraform` | Use `tofu` for OpenTofu |
| `workspace-root` | `github.workspace` | Symlink/path jail |
| `plan-file` | empty | Reuse a trusted local plan (`--plan-file`) |
| `args` | empty | Extra `scan` flags (workflow YAML, not untrusted input) |
| `plugin-cache` | `true` | `actions/cache` + `TF_PLUGIN_CACHE_DIR` |
| `redact-paths` | `true` | CI default |

`--plan-file` ships in v0.4.0+.

## Marketplace

GitHub Marketplace listing needs a public repo, root `action.yml` with branding (this file), and a tagged release. v0.4.0 includes `action.yml`; listing is a GitHub UI step on that release:

1. Open https://github.com/niravraychura/terradrift/releases
2. Use **Publish this Action to the GitHub Marketplace** on that release
3. Add the Marketplace badge/link to the README

The listing URL will be `https://github.com/marketplace/actions/terradrift-scan` (name from `action.yml`).
