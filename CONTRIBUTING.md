# Contributing To Canterbury

Thank you for helping improve Canterbury. The project is early, so small,
focused changes are especially valuable.

## Project Scope

Canterbury currently implements only the sync worker in `sync/`. Planned
components such as the vault service, MCP tools, authorization layer, audit
system, and indexing layer should remain documented as planned architecture
unless they exist in the repository.

## Development Setup

Install repository formatting dependencies:

```bash
npm install
```

Install Go linting tooling:

```bash
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.11.4
```

Install sync worker dependencies:

```bash
npm --prefix sync install
```

Run all checks before submitting changes:

```bash
npm run check
```

This runs repository formatting checks, Go tests, Go linting, sync worker syntax
checking, and ESLint.

## Commit Standards

Use Conventional Commits 1.0.0:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Common commit types include:

- `feat`: new user-facing capability.
- `fix`: bug fix.
- `docs`: documentation-only change.
- `test`: test-only change.
- `refactor`: code change that preserves behavior.
- `chore`: maintenance work.
- `build`: build system or dependency changes.
- `ci`: continuous integration changes.

Mark breaking changes with `!` in the type/scope, or with a
`BREAKING CHANGE:` footer.

Examples:

```text
feat(sync): configure Obsidian sync on first startup
fix(sync): forward SIGTERM to the active ob process
docs: document Docker volume behavior
```

Keep commits atomic. Each commit should represent one coherent piece of work
that can be reviewed, reverted, and built on independently. Split unrelated
changes into separate commits.

## Sync Worker Guidelines

- Preserve the Docker-managed `obsidian-vault:/vault` named volume as the
  portable default.
- Treat host bind mounts such as `./vault:/vault` as local development
  overrides.
- Keep secrets out of source control. Do not commit `sync/.env`.
- Keep child process handling robust: use `close`, forward shutdown signals,
  and avoid leaking secrets in logs.
- Prefer explicit configuration errors and non-zero exit codes.

## Documentation

Keep `README.md` aligned with the current implementation:

- Explain what Canterbury is and why it exists.
- Clearly distinguish implemented functionality from planned architecture.
- Keep setup instructions procedural and concrete.
- Document configuration variables and operational expectations.
- Update troubleshooting when fixing real operational issues.
