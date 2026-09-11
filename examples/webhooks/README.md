# On-call webhooks (PagerDuty / Opsgenie)

TerraDrift has no PagerDuty or Opsgenie SDK. Use `--notify webhook` and map the posted JSON in a small HTTPS adapter you control (or a vendor custom-event transformer). Keep routing keys and API keys in secrets, never in git.

```bash
terradrift scan -d ./terraform/prod --terraform-exec \
  --notify webhook --webhook-url "$WEBHOOK_URL"
```

`$WEBHOOK_URL` must be HTTPS with no userinfo. Generic webhooks still cannot target private/loopback destinations (see `SECURITY.md`). Attribute **values** stay out of the payload unless you pass `--attribute-values`.

## Incoming TerraDrift payload

`--notify webhook` POSTs this shape (`Content-Type: application/json`, `User-Agent: terradrift`):

```json
{
  "scan_id": "a1b2-c3d4-e5f6-7890-abcdef123456",
  "status": "drift_detected",
  "plan_mode": "refresh-only",
  "total_resources_checked": 12,
  "total_changed_resources": 1,
  "message": "Terraform scan completed\nScan ID: a1b2-c3d4-e5f6-7890-abcdef123456\nStatus: drift_detected\nPlan mode: refresh-only\nResources checked: 12\nChanged resources: 1\nBy risk: medium 1\nChanges:\n- MEDIUM  update  aws_instance  aws_instance.web\n  ami"
}
```

Synthetic fixture: [`terradrift-webhook.json`](terradrift-webhook.json). `message` includes capped findings (type, address, actions, attribute paths) — no secret-like attribute values unless `--attribute-values`.

Suggested `status` → page mapping (skip paging on `no_drift` / `skipped` unless you want a resolve event):

| `status` | PagerDuty `payload.severity` | Opsgenie `priority` |
|----------|------------------------------|---------------------|
| `failed` | `critical` | `P1` |
| `drift_detected` | `error` | `P2` |
| `changes_detected` | `warning` | `P3` |

## PagerDuty Events API v2

Your adapter should POST [Events API v2](https://developer.pagerduty.com/docs/ZG9jOjExMDI5NTgx-send-an-alert-event) `enqueue` with a **routing key from a secret**, not from TerraDrift:

- `event_action`: `trigger` (or `resolve` when `status` is `no_drift`)
- `dedup_key`: `terradrift:<scan_id>` (stable per scan; use a root id if you page per Terraform root)
- `payload.summary`: TerraDrift `message` (truncate if you must stay under vendor limits)
- `payload.source`: `terradrift`
- `payload.custom_details`: `scan_id`, `status`, `plan_mode`, counts — never copy `--attribute-values` blobs into details

Mapped example (synthetic routing key): [`pagerduty-events-v2.json`](pagerduty-events-v2.json).

## Opsgenie

Your adapter should POST [Create Alert](https://docs.opsgenie.com/docs/alert-api#create-alert) with an **API key in `Authorization`**, not in the TerraDrift webhook URL:

- `message`: first line of TerraDrift `message` (required, short)
- `alias`: `terradrift:<scan_id>`
- `description`: full `message`
- `source`: `terradrift`
- `details`: same non-secret fields as PagerDuty `custom_details`

Mapped example: [`opsgenie-alert.json`](opsgenie-alert.json).
