# Copilot Instructions for Canterbury

Canterbury is an experimental system for connecting AI agents to an Obsidian
vault through a controlled service layer. The repository currently implements a
Dockerized sync worker (`sync/`), a local Go vault service
(`cmd/vault-service`, `internal/`), and a stateless read-only MCP gateway
(`cmd/mcp-server`).

Refer to `AGENTS.md` for full project context and `CONTRIBUTING.md` for
contributor guidelines. The instructions below summarize the most important
points for coding agents.

## Scope

- Make the smallest change that addresses the task.
- Avoid broad refactors unless explicitly requested.
- Do not add code for planned-but-unimplemented components (write paths,
  indexing, or dedicated MCP audit events). Keep planned architecture documented
  only.
- Do not imply planned components are implemented in code or documentation.

## Development Setup

Run this once to install all Node dependencies, Go modules, and project-local
Go tools (including `golangci-lint`):

```bash
make setup
```

## Running Checks

Run the full check suite before opening a PR:

```bash
make check
```

This executes, in order:

| Check                     | Command                            |
| ------------------------- | ---------------------------------- |
| Prettier formatting       | `npm run format:check`             |
| Protobuf lint             | `npm run proto:lint`               |
| Go formatting             | `npm run format:go:check`          |
| Go tests                  | `npm run test:go`                  |
| Go lint (`golangci-lint`) | `npm run lint:go`                  |
| JS syntax + ESLint        | `npm --prefix sync run check:code` |

Fix formatting automatically with:

```bash
npm run format          # Prettier + Go fmt
npm --prefix sync run lint:fix   # ESLint auto-fix
```

All checks above run in CI on every push and pull request (`.github/workflows/ci.yml`).

## Tests

- Run Go tests with `make test` or `npm run test:go`.
- Add or update unit tests whenever behavior changes or new functionality is
  added. Tests live alongside the code in `internal/` and `cmd/`.
- The CI workflow reports coverage but does not enforce a minimum threshold
  yet (`COVERAGE_THRESHOLD: 0`). Aim to keep coverage at least as high as
  before your change.
- There are no JavaScript tests at this time; JS quality is enforced by ESLint
  and syntax checking.

## Formatting and Style

- `.editorconfig` is authoritative for editor settings (indentation, line
  endings, etc.).
- `.prettierrc` governs Prettier formatting (tabs, single quotes, no semis,
  trailing commas, 100-char print width, LF endings).
- JavaScript uses ES modules; no semicolons; single quotes; tabs (width 2).
- YAML files use 2-space indentation.
- Go files are formatted with `gofmt` and linted with `golangci-lint`
  (config: `.golangci.yml`). Enabled extra linters: `errorlint`, `gosec`,
  `misspell`, `revive`, `unconvert`, `unparam`, `whitespace`.
- Shell scripts use 2-space indentation, LF line endings.
- Makefile targets use tabs.

## Commits

Use [Conventional Commits 1.0.0](https://www.conventionalcommits.org/):

```
<type>[optional scope]: <description>
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `build`, `ci`.
Mark breaking changes with `!` or a `BREAKING CHANGE:` footer.
Keep commits atomic — one coherent unit of work per commit.

Examples:

```
feat(vault-service): add pagination to SearchNotes
fix(sync): forward SIGTERM to the active ob process
docs: update README setup instructions
```

## Documentation

- Update `README.md` when public APIs, configuration variables, setup steps,
  workflows, or developer-facing behavior change.
- Keep `README.md` aligned with The Good Docs Project guidance: explain what
  Canterbury is, distinguish implemented from planned, keep setup procedural.
- Do not document planned components as if they are implemented.

## Security

- Never commit secrets. `sync/.env` is git-ignored; keep it that way.
- Preserve default-deny access: notes without `access.scopes` must not be
  exposed through service interfaces.
- Obsidian credentials must stay isolated to the sync worker. The vault
  service reads a filesystem mirror only.
- Do not introduce code paths that leak credentials in logs.

## Component-Specific Notes

**Sync worker (`sync/`)**

- Preserve the Docker-managed named volume `obsidian-vault:/vault` as the
  portable default. Treat bind mounts as local development overrides only.
- Keep child process handling robust: use `close` events, forward shutdown
  signals, and avoid leaking secrets.
- Prefer explicit error messages and non-zero exit codes on failure.

**Vault service (`cmd/vault-service`, `internal/`)**

- Do not add unimplemented architecture (write paths, audit, indexing) before
  the design has settled.
- Keep `sample-vault/` small and synthetic — useful for exercising read,
  search, tag, and access-scope behavior.

## Open Questions / Recommended Follow-ups

- **Coverage threshold**: `COVERAGE_THRESHOLD` in CI is currently `0`. Consider
  setting a meaningful minimum once the test suite is more complete.
- **JavaScript tests**: there are no JS unit tests. If the sync worker grows in
  complexity, a test framework (e.g., Node's built-in test runner) may be worth
  adding.
- **PR template**: no pull request template exists yet. Adding one could
  reinforce the conventional-commit and checklist expectations above.
