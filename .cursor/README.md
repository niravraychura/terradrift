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
| `github-milestones.mdc` | No empty milestones; attach real work; close on ship |
| `agent-skills.mdc` | Auto-invoke `.agents/skills/` + Serena / Context7 CLI |

## Project skills (`.cursor/skills/`)

TerraDrift-authored skills (this product’s own runbooks):

| Skill | When to use |
|------|-------------|
| [`cut-release`](skills/cut-release/SKILL.md) | Tag/ship a SemVer release (`dev`→`main`→`v*`→verify) |
| [`add-scan-flag`](skills/add-scan-flag/SKILL.md) | Add/change a `scan` / `scan-all` flag end-to-end |

## Auto-invoke skills (`.agents/skills/`)

Installed for Cursor via `npx skills add … -a cursor`. **When to fire** is `.cursor/rules/agent-skills.mdc` (always-on). `skills-lock.json` at the repo root pins hashes.

| Skill | Trigger (see the rule for skips) |
|------|-------------|
| `systematic-debugging` | Bug, failing test, unexpected behavior |
| `test-driven-development` | New/changed Go behavior (not docs/one-liners) |
| `verification-before-completion` | Before claiming done — with `verify-before-done` |
| `brainstorming` | Ambiguous new product behavior |
| `context7-cli` | Current third-party library docs via `ctx7` CLI |

## MCP / CLIs (local)

| Tool | Role |
|------|------|
| Serena (`.cursor/mcp.json`) | Symbol-level navigation for **medium/large** Go changes. Optional local MCP (`uv tool install serena-agent`); GPL-3.0 application, not a TerraDrift dependency. Needs `gopls` on `PATH`. Reload Cursor after MCP edits. Shared project graph: `.serena/memories/` (`mem:core` first). Do not commit `project.local.yml` or `.serena/cache/`. |
| Context7 CLI | `ctx7 library` then `ctx7 docs`. CLI + skill only (no Context7 MCP unless someone asks). |

## Suggested additions later (optional)

- **PR size rule** — soft limit on files/LOC per PR; split otherwise  
- **Hooks** — optional reminder to run `make test` before claiming done  

Do not add secrets, machine paths, or personal tokens under `.cursor/`.
