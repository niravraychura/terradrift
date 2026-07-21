# TerraDrift

TerraDrift is an open-source, self-hosted Terraform drift detection tool.

> **Project status:** TerraDrift is under active development. The current bootstrap release only validates a local Terraform directory. Terraform drift execution and plan parsing are intentionally deferred to the next implementation phase.

## Why TerraDrift exists

Terraform-managed infrastructure can drift when resources are changed outside normal infrastructure-as-code workflows. TerraDrift aims to make drift detection simple to run in local, CI, and self-hosted environments without requiring a hosted SaaS dashboard.

## Planned features

- Local Terraform directory scanning
- Terraform `init`, refresh-only `plan`, and JSON plan parsing
- Clear drift reports for humans and automation
- CI-friendly exit codes
- Configurable logging and output formats
- Future self-hosted dashboard and history storage

## Current CLI usage

```bash
terradrift scan --directory ./terraform/prod
terradrift scan -d ./terraform/prod
```

Current output validates the directory and prints the absolute Terraform path:

```text
TerraDrift scan initialized
Terraform directory: /absolute/path/to/terraform/prod
```

## Local development

Requirements:

- Go 1.25 or newer
- Docker, for container builds
- `golangci-lint`, for local linting

Install dependencies:

```bash
go mod download
```

Run the CLI:

```bash
go run ./cmd/terradrift scan --directory .
```

## Build

```bash
make build
```

The binary is written to `bin/terradrift`.

## Test

```bash
make test
make test-race
make vet
make lint
```

Tests do not require Terraform, cloud credentials, or network access.

## Docker build

```bash
make docker-build
```

The runtime image does not include Terraform yet. Terraform will be added when command execution is implemented.

## Roadmap

1. Implement Terraform CLI runner for `init`, refresh-only `plan`, and `show -json`.
2. Parse Terraform JSON output into TerraDrift drift reports.
3. Add machine-readable output formats and CI-oriented exit codes.
4. Add configuration files and ignore rules.
5. Build dashboard and historical reporting after the CLI foundation is stable.

## Security considerations

- TerraDrift will execute Terraform in user-provided directories in a future release. Review Terraform providers and modules before scanning untrusted code.
- Do not commit credentials, state files, plans, or logs containing secrets.
- Prefer least-privilege cloud credentials for drift detection.
- This bootstrap release does not contact cloud APIs or execute Terraform.

## Contributing

Contributions are welcome. Please keep changes focused, add tests for new behavior, and run formatting, vetting, linting, tests, and builds before opening a pull request.

## License

TerraDrift is released under the MIT License. See [LICENSE](LICENSE).
