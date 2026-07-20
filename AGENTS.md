# Agent Instructions

Use this file to re-establish project context quickly in future AI-assisted
sessions.

## Project Context

The project is **Canterbury**.

Canterbury is an experimental system for connecting AI agents to an Obsidian
vault through a controlled service layer. The long-term goal is to allow agents
to read, search, and eventually write vault content while enforcing explicit
access policies and recording an independent audit trail outside the vault.

The repository currently implements the **sync worker**, a local **vault
service** with scoped read and search paths, and a stateless HTTP **MCP server**
for those paths.

## Current Implemented Components

The sync worker lives in `sync/` and:

- Runs as a Node.js container entrypoint.
- Uses `obsidian-headless` through the `ob` CLI.
- Writes an Obsidian auth token to the location expected by `ob`.
- Runs `ob sync-status` and performs `ob sync-setup` only when needed.
- Runs `ob sync --continuous`.
- Forwards shutdown signals to the child `ob` process.

The default Docker Compose setup uses a Docker-managed named volume:

```yaml
volumes:
  - obsidian-vault:/vault
```

Keep this as the portable default. Treat host bind mounts such as `./vault:/vault`
as explicit local development overrides because they introduce host filesystem
permission concerns.

The vault service lives under `cmd/vault-service` and `internal/` and:

- Reads Markdown notes from a local filesystem vault mirror.
- Parses YAML frontmatter, note tags, and note-declared access scopes.
- Exposes Connect/gRPC health, reflection, `ReadNote`, and `SearchNotes`.
- Validates bearer JWTs and maps verified issuer/subject pairs to local scopes.
- Enforces default-deny access by requiring a matching note `access.scopes`
  value.
- Strips reserved access-policy frontmatter from returned note properties.
- Writes mandatory read, search, and authentication audit events outside the
  vault.

The MCP server lives under `cmd/mcp-server` and:

- Exposes stateless Streamable HTTP with JSON responses at `POST /mcp`.
- Publishes only the allowlisted `read_note` and `search_notes` tools.
- Requires one bearer assertion per request and forwards it to the internal
  vault Connect service with request and trace correlation metadata.
- Relies on the vault service for authentication, authorization, and mandatory
  operation audit records.

Its Streamable HTTP handler, bearer middleware, tool allowlist, and metadata
forwarding interceptor live in `internal/interfaces/mcphttp`; the command owns
configuration, client wiring, signals, and HTTP lifecycle.

The repository includes `sample-vault/` with small fake notes for local service
testing and demos.

## Planned Architecture

The broader system is expected to include:

- **Sync worker**: trusted component with Obsidian credentials.
- **Vault service**: Go service that exposes controlled access to vault data.
  Read, search, authentication, and audit integration are implemented; write
  paths and indexing remain incomplete.
- **MCP tools**: read-only AI-facing tool interface for querying vault data.
- **Authorization and classification**: default-deny access based on
  note-declared access scopes and caller principal scopes.
- **Audit system**: independent append-only audit records outside the vault.
- **Indexing layer**: parsed metadata/search cache, likely SQLite first.
- **Plugin and operation framework**: extensible processing over vault events.

Do not imply these planned components are implemented unless they exist in the
repository.

## Security Model

Maintain these assumptions:

- The sync worker is trusted and has Obsidian account credentials.
- Vault services must not receive Obsidian account credentials.
- AI agents must not access vault files directly.
- Access should be default deny.
- Only notes declaring allowed `access.scopes` should become available through
  controlled service interfaces.
- AI agents and future integrations should be treated as principals with scopes.
- Missing `access.scopes` should remain default deny.
- Future write operations must not commit successfully without a corresponding
  independent audit record.

The planned frontmatter shape is:

```yaml
access:
  scopes:
    - personal-agent
```

## Style And Tooling

Follow the repository formatting rules:

- `.editorconfig` is authoritative for editor behavior.
- `.prettierrc` configures repository-wide Prettier formatting.
- `.golangci.yml` configures Go linting.
- JavaScript uses ES modules.
- Prefer tabs with width 2 for JavaScript and JSON.
- Prefer LF line endings.
- Do not add semicolons.
- Use single quotes.

For repository changes, run from the repository root:

```bash
make check
```

This runs:

- `prettier --check`
- `buf lint`
- `gofmt` checks
- `go test ./cmd/... ./internal/... ./gen/go/...`
- `golangci-lint run ./cmd/... ./internal/...`
- `node --check sync.js`
- `eslint .`

Use root `npm run format` and sync `npm run lint:fix` when appropriate.
`npm run check` is also available when the required local tools are already on
`PATH`.

## Documentation Standards

Keep `README.md` aligned with The Good Docs Project README guidance:

- Explain what Canterbury is and why it exists.
- Clearly distinguish implemented functionality from planned architecture.
- Keep setup instructions procedural and concrete.
- Document configuration variables and operational expectations.
- Update troubleshooting when fixing real operational issues.

## Development Guidance

When making changes:

- Preserve the named-volume Docker default unless the user explicitly asks for a
  development bind mount.
- Avoid broad refactors in the sync worker unless they materially improve
  correctness, reliability, or maintainability.
- Keep secrets out of source control. `sync/.env` must remain ignored.
- Prefer explicit error messages and positive process exit codes.
- Keep child process handling robust: use `close`, forward signals, and avoid
  leaking secrets in logs.
- Do not add future architecture code before the design has settled.
- Keep `sample-vault/` content small, fake, and useful for exercising current
  read, search, tag, and authorization behavior.

## Commit Standards

Use Conventional Commits 1.0.0 for commit messages:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Prefer common types such as `feat`, `fix`, `docs`, `test`, `refactor`, `chore`,
`build`, and `ci`. Use `feat` for new user-facing capability, `fix` for bug
fixes, and mark breaking changes with `!` in the type/scope or a
`BREAKING CHANGE:` footer.

Keep commits atomic. Each commit should represent one coherent piece of work
that can be reviewed, reverted, and built on independently. If a change mixes
unrelated concerns, split it into multiple commits that build on each other in a
clear order.

## Useful Files

- `README.md`: project overview, usage, and roadmap.
- `Dockerfile.sync`: sync worker image.
- `docker-compose.yml`: local sync worker deployment.
- `.env.example`: local vault service demo configuration.
- `sample-vault/`: fake notes for local service tests and demos.
- `cmd/vault-service/main.go`: local vault service executable.
- `internal/`: Go domain, application, adapter, and interface packages.
- `docs/maintenance.md`: pinned generator and vendored schema update process.
- `bruno/canterbury/`: sample Connect/gRPC requests.
- `package.json`: repository-level formatting and check orchestration.
- `.prettierrc`: repository-wide Prettier config.
- `.golangci.yml`: Go linting config.
- `sync/sync.js`: sync worker entrypoint.
- `sync/package.json`: sync worker tooling and dependencies.
- `sync/eslint.config.js`: ESLint flat config.
- `.editorconfig`: repository editor defaults.
