# TerraDrift Conftest Policy

`terradrift.rego` denies destructive drift and replacements under a path containing `prod`.

Run the sample with Conftest:

```bash
conftest test --parser json -p examples/policy examples/policy/sample-report.json
```

Run it through TerraDrift's policy hook:

```bash
terradrift scan \
  --directory ./terraform/prod \
  --redact-paths \
  --policy-command conftest \
  --policy-arg test \
  --policy-arg --parser \
  --policy-arg json \
  --policy-arg=-p \
  --policy-arg examples/policy \
  --policy-arg=-
```

The report does not contain resource tags or configuration attributes. Add policy rules only for fields TerraDrift emits.
