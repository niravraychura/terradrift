# Cost Adapters

TerraDrift sends the current drift report JSON on standard input to `--cost-command`. The command must return only this JSON shape on standard output:

```json
{
  "resource_costs": [
    {"address": "aws_instance.web", "monthly_delta": "+$12.34/mo"}
  ]
}
```

`address` must exactly match a Terraform resource address in the drift report. TerraDrift ignores unknown or empty entries.

## Custom Adapter

Use an executable that reads the report from standard input, calculates cost deltas, and writes the normalized JSON to standard output. Pass every argument separately; TerraDrift never invokes a shell.

```bash
terradrift scan \
  --cost-command /usr/local/bin/terradrift-cost-adapter \
  --cost-arg --environment \
  --cost-arg production
```

Keep API keys in the adapter's environment or secret manager, not in command arguments. Adapter output is capped at 256 KiB and command errors are redacted before display.

## Infracost

Infracost produces its own detailed JSON from a Terraform directory or plan JSON. Run it separately, then adapt its resource output to TerraDrift's normalized contract:

```bash
infracost breakdown --path ./terraform/prod --format json --out-file infracost.json
```

The adapter should map each Infracost resource to its Terraform address and calculate the approved monthly delta before returning `resource_costs`. Do not pass `infracost` directly as `--cost-command`: it does not consume TerraDrift's drift-report input or produce the required normalized output.
