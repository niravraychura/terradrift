# Configuration Examples

- `local.json`: local Terraform execution, history, and dashboard output.
- `ci.json`: redacted CI output with history throttling, policy, cost, audit adapters, and an external-command allowlist. Always set both `allowed_commands` and `trusted_command_dirs` in CI; empty allowlists mean local trust only.
- `../multi-root/`: JSON manifest with per-root profile/plan_mode/var-files plus a scheduled Slack + severity-gate workflow.

Copy an example to `.terradrift.json` and replace paths and adapter names. Keep webhook URLs and API tokens in environment variables or a secret manager, not in config files.
