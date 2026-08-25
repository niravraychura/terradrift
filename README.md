# TerraDrift

[![CI](https://github.com/niravraychura/terradrift/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/niravraychura/terradrift/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/niravraychura/terradrift?include_prereleases&sort=semver)](https://github.com/niravraychura/terradrift/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Self-hosted Terraform / OpenTofu drift detection CLI.**  
Run it on a laptop, in CI, or on a cron runner. No SaaS required.

TerraDrift runs `terraform plan` (or OpenTofu), turns the plan into a clear report, and can notify Slack/Teams/webhooks, write history/dashboards, and gate on policy.

> **Important:** Always pass `--terraform-exec` for a real drift scan. Without it, TerraDrift only checks the directory and emits a bootstrap placeholder report.

---

## Contents

1. [Quick start](#1-quick-start)
2. [Understand the result](#2-understand-the-result)
3. [Use in CI (scheduled)](#3-use-in-ci-scheduled)
4. [Common options](#4-common-options)
5. [Scan many roots](#5-scan-many-roots)
6. [Configuration file](#6-configuration-file)
7. [Docker](#7-docker)
8. [How a Terraform-backed scan works](#8-how-a-terraform-backed-scan-works)
9. [Security defaults](#9-security-defaults)
10. [More documentation](#10-more-documentation)
11. [Develop from source](#11-develop-from-source)
12. [Contributing & license](#12-contributing--license)

---

## 1. Quick start

### Step 1 — Install TerraDrift

Download a binary from [GitHub Releases](https://github.com/niravraychura/terradrift/releases) (Linux amd64 / macOS arm64), or build from source:

```bash
git clone https://github.com/niravraychura/terradrift.git
cd terradrift
make build
./bin/terradrift --help
./bin/terradrift --version   # local builds report "dev"; release binaries use the tag
```

### Step 2 — Have Terraform (or OpenTofu) ready

- `terraform` or `tofu` on your `PATH`
- A local Terraform root (any folder with `.tf` files — **not** required to live inside this repo)
- Credentials / backend access so `terraform plan` can run (same as you would for a normal plan)

### Step 3 — Run a real drift scan

```bash
# From inside a Terraform root:
terradrift scan --terraform-exec

# Or point at a directory:
terradrift scan --directory ./terraform/prod --terraform-exec
```

Default plan mode is **`refresh-only`** (out-of-band infrastructure drift vs state).  
To see unapplied config changes as well:

```bash
terradrift scan -d ./terraform/prod --terraform-exec --plan-mode normal
```

OpenTofu:

```bash
terradrift scan -d ./terraform/prod --terraform-exec --terraform-bin tofu
```

### Step 4 — Optional: write a starter config

```bash
terradrift init --directory ./terraform/prod --terraform-exec --history-dir .terradrift-history
terradrift scan --config .terradrift.json
```

---

## 2. Understand the result

### Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | Success, no actionable drift/changes (or findings below `--failure-severity`) |
| `1` | Scan failed (including policy publish-gate failure) |
| `2` | Drift / changes detected at or above the failure threshold |

### Example table output

```text
TerraDrift scan initialized
Status: drift_detected
Plan mode: refresh-only
Resources checked: 144
Changed resources: 2

CRITICAL  delete,create  module.ecs.aws_ecs_task_definition.td
  reason: replace_because_cannot_update
  cpu: "256" -> "512"
```

Machine-readable outputs:

```bash
terradrift scan -d ./terraform/prod --terraform-exec --output json
terradrift scan -d ./terraform/prod --terraform-exec --output junit
terradrift scan -d ./terraform/prod --terraform-exec --output sarif
terradrift scan -d ./terraform/prod --terraform-exec --output prometheus
```

Report JSON stability notes: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## 3. Use in CI (scheduled)

TerraDrift does **not** need to live in your infra repo. Checkout the Terraform repo, then run TerraDrift in the same job.

Minimal pattern:

```yaml
- uses: hashicorp/setup-terraform@v4
- name: Drift scan
  run: terradrift scan --directory ./terraform/prod --terraform-exec --output json
```

Full scheduled examples:

- GitHub Actions: [`examples/github-actions/terradrift-scheduled.yml`](examples/github-actions/terradrift-scheduled.yml)
- Multi-root + Slack: [`examples/github-actions/terradrift-scheduled-multi-root.yml`](examples/github-actions/terradrift-scheduled-multi-root.yml)
- Cron: [`examples/cron/terradrift.cron`](examples/cron/terradrift.cron)

Pin TerraDrift, Terraform/OpenTofu, and provider versions. Keep cloud credentials and webhook URLs in CI secrets.

---

## 4. Common options

### Notify humans

```bash
terradrift scan -d ./terraform/prod --terraform-exec \
  --notify slack --slack-webhook-url "$SLACK_WEBHOOK_URL"

terradrift scan -d ./terraform/prod --terraform-exec \
  --notify teams --teams-webhook-url "$TEAMS_WEBHOOK_URL"

terradrift scan -d ./terraform/prod --terraform-exec \
  --notify webhook --webhook-url "$WEBHOOK_URL"
```

### History + HTML dashboard

```bash
terradrift scan -d ./terraform/prod --terraform-exec \
  --history-dir .terradrift-history \
  --dashboard-html terradrift-report.html
```

Browse history locally (loopback only):

```bash
terradrift serve --history-dir .terradrift-history
```

### Fail CI only on high-severity findings

```bash
terradrift scan -d ./terraform/prod --terraform-exec --failure-severity high
```

Replacements are `critical`, deletes `high`, creates/updates `medium`.

### Workspace and variables

```bash
terradrift scan -d ./terraform/prod --terraform-exec \
  --workspace prod \
  --var-file prod.tfvars \
  --var 'region=us-east-1'
```

### Policy publish gate (before history / notify)

```bash
terradrift scan -d ./terraform/prod --terraform-exec \
  --policy-command conftest \
  --policy-arg test \
  --policy-arg -
```

Example policy pack: [`examples/policy`](examples/policy/README.md).

### Cost / audit adapters

```bash
terradrift scan -d ./terraform/prod --terraform-exec \
  --cost-command infracost --cost-arg breakdown --cost-arg --format=json
```

See [docs/COST_ADAPTERS.md](docs/COST_ADAPTERS.md) and [docs/AUDIT_ADAPTERS.md](docs/AUDIT_ADAPTERS.md).

---

## 5. Scan many roots

**Option A — manifest file** (one root per line):

```text
# terraform-roots.txt
environments/development
environments/production
```

```bash
terradrift scan-all --manifest terraform-roots.txt --concurrency 4 --terraform-exec --output json
```

**Option B — JSON manifest** (per-root workspace / vars / profile):

```json
{
  "version": 1,
  "roots": [
    {"directory": "terraform/dev", "profile": "development", "var_files": ["dev.tfvars"]},
    {"directory": "terraform/prod", "plan_mode": "refresh-only", "workspace": "prod"}
  ]
}
```

**Option C — discover** roots under a workspace:

```bash
terradrift scan-all --discover . --terraform-exec --concurrency 4
```

More detail and examples: [`examples/multi-root`](examples/multi-root).

`scan-all` uses the same per-root delivery path as `scan` (history, notify, policy, ignore/owners, GitHub, artifacts, audit-log). Shared `--dashboard-html` / `--github-pr` still apply once per root — prefer `dashboard-index` and comment upsert for quieter multi-root CI.

Cross-root HTML index from history:

```bash
terradrift dashboard-index --history-dir .terradrift-history --output terradrift-index.html
```

---

## 6. Configuration file

```bash
terradrift init
terradrift scan --config .terradrift.json --profile production
```

Example profile:

```json
{
  "profiles": {
    "production": {
      "directory": "./terraform/prod",
      "output": "json",
      "terraform_exec": true,
      "plan_mode": "refresh-only",
      "redact_paths": true
    }
  }
}
```

- Schema: [`docs/terradrift.schema.json`](docs/terradrift.schema.json)
- Ready-made configs: [`examples/config`](examples/config/README.md)

CLI flags always override config values.

---

## 7. Docker

Image: `ghcr.io/niravraychura/terradrift:<version>` (also `latest` from releases).

The runtime image does **not** include Terraform. For `--terraform-exec`, mount a binary or extend the image:

```dockerfile
FROM ghcr.io/niravraychura/terradrift:v0.1.0
USER root
RUN apk --no-cache add curl unzip \
  && curl -fsSLo /tmp/terraform.zip https://releases.hashicorp.com/terraform/1.10.5/terraform_1.10.5_linux_amd64.zip \
  && unzip /tmp/terraform.zip -d /usr/local/bin \
  && rm /tmp/terraform.zip
USER terradrift:terradrift
```

Do not bake cloud credentials into the image.

---

## 8. How a Terraform-backed scan works

With `--terraform-exec`, each scan roughly:

1. Validates the directory and takes a **local** scan lock (`.terradrift-scan.lock` on that host).
2. Runs `terraform init` (unless `--skip-terraform-init`) with `-lockfile=readonly` — a committed `.terraform.lock.hcl` is required.
3. Runs `plan` (`refresh-only` or `normal`) with `-detailed-exitcode`.
4. Runs `terraform show -json`, parses the plan, builds the TerraDrift report.
5. Writes stdout, then optional policy gate → history / dashboard / notifications.

`refresh-only` statuses: `no_drift` / `drift_detected`.  
`normal` statuses: `no_changes` / `changes_detected` (config drift is not labelled as infrastructure drift).

Compare both modes when unsure whether a finding is out-of-band change vs unapplied config.

---

## 9. Security defaults

- Trusted-runner model: TerraDrift runs Terraform with whatever credentials the process has.
- Attribute **paths** are always available; **values** in history/policy/notifications are paths-only unless `--attribute-values` is set. Sensitive values stay `[REDACTED]`.
- Prefer read-only cloud credentials for refresh-only scans.
- Keep webhooks and tokens in a secret manager / CI secrets — never commit them.
- Full posture and reporting: [SECURITY.md](SECURITY.md) · IAM notes: [docs/DRIFT_SCAN_IAM.md](docs/DRIFT_SCAN_IAM.md)

---

## 10. More documentation

| Topic | Doc |
| --- | --- |
| Architecture & report JSON | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Roadmap / out of scope | [docs/ROADMAP.md](docs/ROADMAP.md) |
| Release cycle (`dev` → `main` → tag) | [docs/RELEASE.md](docs/RELEASE.md) |
| Changelog | [CHANGELOG.md](CHANGELOG.md) |
| Cost adapters | [docs/COST_ADAPTERS.md](docs/COST_ADAPTERS.md) |
| Audit adapters | [docs/AUDIT_ADAPTERS.md](docs/AUDIT_ADAPTERS.md) |
| All `scan` flags | `terradrift scan --help` |

Advanced features (baselines, ignore rules, owner routing, GitHub PR/issue comments, approvals, artifact upload) are configured via `.terradrift.json` / flags — see `terradrift scan --help` / `terradrift scan-all --help` and [examples/config](examples/config/README.md).

---

## 11. Develop from source

Requirements: Go (see `go.mod`), Make; Terraform only if you exercise `--terraform-exec` locally.

```bash
make build          # → bin/terradrift
make test
make ci             # fmt check, vet, test, race, vuln, lint
```

---

## 12. Contributing & license

- PRs target **`dev`** — see [CONTRIBUTING.md](CONTRIBUTING.md)
- Security reports: [SECURITY.md](SECURITY.md)
- Agents / Cursor defaults: [AGENTS.md](AGENTS.md)
- License: [MIT](LICENSE)
