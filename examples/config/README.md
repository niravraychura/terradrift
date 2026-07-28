# Configuration Examples

- `local.json`: local Terraform execution, history, and dashboard output.
- `ci.json`: redacted CI output with history throttling, policy, cost, audit adapters, and an external-command allowlist.

Copy an example to `.terradrift.json` and replace paths and adapter names. Keep webhook URLs and API tokens in environment variables or a secret manager, not in config files.
