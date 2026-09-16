# Task completion

Do not claim done without fresh evidence (`verification-before-completion` + `.cursor/rules/verify-before-done.mdc`).

1. Smallest relevant test first: `go test ./path/under/change` (or `go test ./...`).
2. Multi-package: `make test`. Release/CI-shaped: `make ci` (needs golangci-lint + network for vuln).
3. Docs-only: skip heavy CI; still check edited links/paths.
4. After push: PR into **`dev`**; `gh pr checks <n> --watch` until `test` completes. Fix on the same branch if red.
5. Reply with PR URL + CI outcome. Do not merge to `main` or cut tags unless the user asked for a release (`docs/RELEASE.md`).

Never claim “tests pass” without running them or quoting green CI.
