# Commands

Prefer Make targets over raw go when the Makefile already wraps flags.

```bash
make build          # bin/terradrift
make test           # go test ./...
make test-race
make fmt            # gofmt -w via rg file list
make vet
make lint           # golangci-lint run
make ci             # fmt-check + vet + test + race + vuln + lint (needs golangci-lint + network for vuln)
make run            # go run ./cmd/terradrift --help
```

Darwin: same as unix for git/make/go. `rg` is required for `make fmt` / `make ci` fmt check. `gh` GraphQL from this environment needs unrestricted permissions on first call (sandbox Forbidden is not “GraphQL unavailable”).

Serena: `serena memories check` from repo root after memory edits.
