# Configuration Examples

- `local.json`: local Terraform execution, history, dashboard output, plus sample `baseline_rules` / `ignore_rules`.
- `ci.json`: redacted CI output with history throttling, policy, cost, audit adapters, and an external-command allowlist. Always set both `allowed_commands` and `trusted_command_dirs` in CI; empty allowlists mean local trust only.
- `../multi-root/`: JSON manifest with per-root profile/plan_mode/var-files plus a scheduled Slack + severity-gate workflow.

Each example sets `"$schema"` to [`docs/terradrift.schema.json`](../../docs/terradrift.schema.json) so editors can validate the file. `terradrift init` writes the same field.

Copy an example to `.terradrift.json` and replace paths and adapter names. Keep webhook URLs and API tokens in environment variables or a secret manager, not in config files.
