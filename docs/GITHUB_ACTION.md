# GitHub Action

TerraDrift ships a composite Action at the repository root (`action.yml`).

```yaml
- uses: hashicorp/setup-terraform@v4
  with:
    terraform_wrapper: false
- uses: niravraychura/terradrift@v1.1.1
  with:
    directory: ./terraform/prod
```

`uses: @vX.Y.Z` installs that tag; omit `version:`. A SHA or branch pin is not a `v*` tag, so set `version:` to the matching GitHub Release:

```yaml
- uses: niravraychura/terradrift@188034031ee2b9ad1ed042d17cd811d5b3767292 # v1.1.1
  with:
    version: v1.1.1
    directory: ./terraform/prod
```

The Action always passes `--terraform-exec`, fails if `terraform` / `tofu` is missing, sets `--workspace-root` to `github.workspace`, and caches providers in `TF_PLUGIN_CACHE_DIR` unless `plugin-cache: false`.

Happy-path GitHub delivery uses `GITHUB_TOKEN` from the job (`--github-pr` / `--github-issue-after` / `--skip-if-open-pr`). `--github-issue-after` upserts one issue per root+fingerprint and closes it when that root scans clean. `--skip-if-open-pr` skips when an open PR changes files under that Terraform root (`status: skipped`, exit 0). Do not put a PAT in the workflow. Cloud auth is OIDC — [DRIFT_SCAN_IAM.md](DRIFT_SCAN_IAM.md).

On GitHub Enterprise Server or GHEC with data residency, GitHub Actions already sets `GITHUB_API_URL` (for example `https://github.example.com/api/v3`). TerraDrift uses that host for PR comments and persistent issues. The URL must be HTTPS with no userinfo. Generic `--notify webhook` still blocks private destinations.

## Inputs

| Input | Default | Notes |
|-------|---------|--------|
| `version` | action `v*` ref | Required when `uses:` is a SHA or branch. Tag pins (`@vX.Y.Z`) can omit it. |
| `directory` | `.` | Terraform root |
| `terraform-bin` | `terraform` | Use `tofu` for OpenTofu, or `terragrunt` for a Terragrunt root |
| `workspace-root` | `github.workspace` | Symlink/path jail |
| `plan-file` | empty | Reuse a trusted local plan (`--plan-file`) |
| `args` | empty | Extra `scan` flags (workflow YAML, not untrusted input) |
| `plugin-cache` | `true` | `actions/cache` + `TF_PLUGIN_CACHE_DIR` |
| `redact-paths` | `true` | CI default |

`--plan-file` ships in v0.4.0+.

## Marketplace

Listed: [TerraDrift Scan](https://github.com/marketplace/actions/terradrift-scan) (free Action listing). Pin `uses: niravraychura/terradrift@v1.1.1`.

To refresh the listing after a new tag, edit that release and keep **Publish this release to the GitHub Marketplace** checked.
