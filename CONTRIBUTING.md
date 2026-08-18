# Contributing to TerraDrift

Thanks for contributing. TerraDrift is a self-hosted Terraform/OpenTofu drift detection CLI.

## Branch model

1. Branch from **`dev`**.
2. Open pull requests **into `dev`** (not `main`).
3. Maintainers promote **`dev` → `main`** when CI is green, then tag releases on `main`.

See [docs/RELEASE.md](docs/RELEASE.md) for the full release policy.

## Development setup

Requirements: Go (see `go.mod`), Make, and optionally Terraform/OpenTofu for integration-style checks.

```bash
make build
make test
make ci   # format check, vet, test, race, vuln, lint (needs golangci-lint + network for vuln)
```

Useful targets: `make fmt`, `make vet`, `make lint`, `make test-race`.

## Pull requests

- Keep changes focused; match existing style and docs tone.
- Add or update tests for behavior changes.
- Update README or `docs/` when user-facing behavior changes.
- Use the PR template checklist.
- Do **not** commit credentials, Terraform state, plan files, webhook URLs, or realistic secret-like fixture strings. Prefer obviously synthetic placeholders (for example `REDACTED_TEST_VALUE`, `https://example.test/hook`).
- Do not invent a release process that contradicts [docs/RELEASE.md](docs/RELEASE.md).

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md). Do not file public issues that include exploit details or secrets.

## Code review expectations

- CI must pass on the PR.
- Sensitive paths (workflows, release, security docs) may require CODEOWNERS review once configured.

## License

By contributing, you agree that your contributions are licensed under the MIT License ([LICENSE](LICENSE)).
