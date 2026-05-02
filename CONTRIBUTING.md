# Contributing To Canterbury

Thank you for helping improve Canterbury. The project is early, so small,
focused changes are especially valuable.

## Project Scope

Canterbury currently implements a Dockerized sync worker in `sync/` and an
initial local Go vault service with scoped `ReadNote` and `SearchNotes` RPCs.
Planned components such as MCP tools, independent audit logging, write
operations, and indexing should remain documented as planned architecture unless
they exist in the repository.

## Development Setup

The recommended setup path installs Node dependencies, Go modules, and
project-local Go tools:

```bash
make setup
```

Run all checks before submitting changes:

```bash
make check
```

You can run lower-level commands directly when the required tools are already
installed:

```bash
npm install
npm --prefix sync install
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

## Vault Service Guidelines

- Preserve default-deny access behavior. Notes without `access.scopes` should
  not be exposed through controlled service interfaces.
- Keep Obsidian account credentials isolated to the sync worker. The vault
  service should read a filesystem mirror only.
- Keep sample vault content small, fake, and useful for demonstrating read,
  search, tag, and authorization behavior.
- Do not add future architecture code before the design has settled.

## Documentation

Keep `README.md` aligned with the current implementation:

- Explain what Canterbury is and why it exists.
- Clearly distinguish implemented functionality from planned architecture.
- Keep setup instructions procedural and concrete.
- Document configuration variables and operational expectations.
- Update troubleshooting when fixing real operational issues.
