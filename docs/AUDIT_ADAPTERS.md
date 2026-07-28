# Cloud Audit Adapters

Use `--audit-command` to correlate drifted Terraform addresses with CloudTrail, Azure Activity Log, or GCP Audit Log events. TerraDrift sends the drift report JSON on standard input and expects this JSON on standard output:

```json
{
  "resource_events": [
    {
      "address": "aws_instance.web",
      "events": [{"provider": "aws", "actor": "alice", "occurred_at": "2026-07-28T12:00:00Z", "summary": "updated instance"}]
    }
  ]
}
```

The adapter owns cloud authentication and address-to-audit-resource mapping. Pass each argument with `--audit-arg`; TerraDrift never invokes a shell. Output is capped at 256 KiB and failures are redacted.
