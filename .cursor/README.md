# Cursor defaults (TerraDrift)

Committed rules under `.cursor/rules/` so every agent session inherits the same bar.

| Rule | Purpose |
|------|---------|
| `ponytail.mdc` | Lazy-senior / YAGNI / smallest correct diff ([Ponytail](https://github.com/DietrichGebert/ponytail)) |
| `engineering-quality.mdc` | Quality, readability, maintainability, testability, performance, security |
| `verify-before-done.mdc` | Local tests before push; GitHub CI check after push |
| `ask-before-assuming.mdc` | Clarify ambiguity; otherwise recommend and proceed |
| `terradrift-workflow.mdc` | `dev`/`main` flow, docs ownership, secret-safe fixtures |
| `go-cli.mdc` | Go CLI fail-closed / policy / attribute conventions (when editing `*.go`) |
| `docs-with-flags.mdc` | CLI/flag changes must update README help path + `--help` |
| `fixtures-no-secrets.mdc` | Tests/examples: synthetic placeholders only |

## Suggested additions later (optional)

- **PR size rule** — soft limit on files/LOC per PR; split otherwise  
- **Project skill** (`.cursor/skills/`) — short “cut a release” or “add scan flag” runbooks if those become frequent  
- **Hooks** — optional reminder to run `make test` before claiming done  

Do not add secrets, machine paths, or personal tokens under `.cursor/`.
