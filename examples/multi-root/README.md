# Multi-root scheduled drift example

End-to-end pattern: JSON manifest with per-root settings, Slack notifications, and a CI severity gate.

## Layout

| File | Purpose |
|------|---------|
| `terraform-roots.json` | Roots with per-root `plan_mode` / `profile` / `var_files` |
| `terradrift.json` | Named profiles and shared delivery defaults |
| `../github-actions/terradrift-scheduled-multi-root.yml` | Scheduled GitHub Actions workflow |

## Local dry run

```bash
# Provide the webhook from a secret manager / CI secret — never commit the URL.
: "${SLACK_WEBHOOK_URL:?set SLACK_WEBHOOK_URL in your environment}"

terradrift scan-all \
  --manifest examples/multi-root/terraform-roots.json \
  --config examples/multi-root/terradrift.json \
  --terraform-exec \
  --workspace-root "$PWD" \
  --redact-paths \
  --notify slack \
  --slack-webhook-url "$SLACK_WEBHOOK_URL" \
  --output json
```

The scheduled workflow applies a **high** severity gate on top of the aggregate exit code so medium-only findings can warn without failing the job.

Notes:

- Keep webhook URLs in secrets, never in the JSON files.
- Attribute values stay paths-only in history/notifications unless you pass `--attribute-values`.
- Per-root `profile` values are resolved from `--config`.