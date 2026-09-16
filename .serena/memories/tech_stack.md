# Tech stack

- **Language:** Go `1.26` (`toolchain go1.26.6` in `go.mod`). Module `github.com/niravraychura/terradrift`.
- **CLI:** `github.com/spf13/cobra`. Keep extra module deps rare (Ponytail).
- **Build:** `make` → `go build -trimpath` to `bin/terradrift` from `./cmd/terradrift`. Version via `-X main.version`.
- **Lint:** `.golangci.yml` + `golangci-lint run`.
- **Vuln:** `make vuln` uses `GOTOOLCHAIN=go1.26.5` + `govulncheck@latest` (pin differs from go.mod toolchain — do not “fix” without checking Makefile).
- **Format:** `gofmt` (not goimports as the gate). `make ci` requires `gofmt -l` empty; `make fmt` uses `rg --files -g '*.go'`.
- **Packaging:** GitHub Release archives, `scripts/install.sh`, official Action, GHCR. Not a library API.
- **Serena:** LSP `go`; needs `gopls` on PATH. Optional MCP; GPL-3.0 app, not a runtime dep.
