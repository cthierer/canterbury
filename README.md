# Canterbury

Canterbury is an experimental system for connecting AI agents to an Obsidian
vault through a controlled service layer. The long-term goal is to let agents
read, search, and eventually write vault content while enforcing explicit access
policies and recording an independent audit trail of every interaction.

This repository currently implements a Dockerized sync worker and an early local
Go vault service with scoped `ReadNote` and `SearchNotes` RPCs.

## Project Status

Canterbury is in early development. The current implementation includes:

- A Node-based sync worker container.
- `obsidian-headless` integration.
- Docker Compose configuration.
- A local Go vault service that reads and searches scoped Markdown notes from a
  filesystem vault mirror.
- Scope-based authorization using note-declared access scopes and a fixed local
  principal configured by environment variable.
- Connect/gRPC health, reflection, `ReadNote`, and `SearchNotes` handlers.
- Repository formatting, test, and linting tooling.

Planned or incomplete components include:

- MCP-compatible tools for AI agents.
- Independent audit logging.
- Indexing and plugin-style vault operations.

## Project Description

Canterbury aims to act as secure middleware between AI agents and an Obsidian
vault. AI agents should not read or write vault files directly. Instead, agents
will interact with a service that authorizes requests, exposes structured query
interfaces, and records actions outside the vault.

The sync worker is the trusted component that mirrors the vault. The vault
service consumes that mirrored vault without receiving Obsidian account
credentials.

## Who This Project Is For

Use Canterbury if you want to explore AI-assisted workflows over an Obsidian
vault while keeping clear boundaries around access, authorization, and audit
records.

The current sync worker is useful if you want to run Obsidian Sync in Docker on
a server, NAS, homelab machine, or cloud container environment.

The current vault service is useful for local development of controlled read
access over a filesystem mirror of the vault. It is not yet a complete
agent-facing service because request authentication, audit logging, and MCP
tools are still planned.

Canterbury is not a replacement for an Obsidian Sync subscription. The sync
worker requires valid Obsidian Sync credentials and an existing remote vault.

## Architecture

The intended system has several components:

- **Sync worker**: Mirrors an Obsidian vault using `obsidian-headless`.
- **Vault service**: Provides the primary controlled interface to vault data.
- **Authorization and classification**: Grants access only to explicitly scoped
  notes when the caller principal has matching scopes.
- **Audit system**: Records operations outside the vault in an append-only log.
- **Indexing layer**: Parses notes into searchable metadata.
- **Plugin and operation framework**: Runs extensible processing over vault
  events and content.

The current repository implements the sync worker and the first vault service
read and search paths. MCP tools, indexing, and audit logging are not
implemented yet.

See [Canterbury Architecture](docs/architecture.md) for the planned Go package
structure and dependency boundaries.

## Access Model

The access model is default deny. Notes opt in to controlled service exposure by
declaring access scopes in frontmatter:

```yaml
access:
  scopes:
    - personal-agent
    - public-site
```

Missing `access.scopes` means the note is not available through controlled
service interfaces. The current vault service uses one local principal whose
scopes come from `VAULT_SERVICE_AUTH_SCOPES`. Future callers, including AI
agents, website renderers, and administrative tools, will operate as
individually authenticated principals with scopes.

## Project Dependencies

To run the current sync worker, you need:

- Docker with Docker Compose.
- An Obsidian account with Sync enabled.
- An Obsidian auth token.
- The name or ID of the remote vault to sync.
- The end-to-end encryption password for the remote vault.

To develop Canterbury or run the local vault service, you also need:

- Go 1.26 or newer.
- `golangci-lint` 2.11.4 or a compatible 2.x release.
- Node.js 24 or a compatible version supported by the sync dependencies.
- npm.
- Make.

Install development dependencies and tools:

```bash
make setup
```

Run the full repository check:

```bash
make check
```

## Install Canterbury

1. Clone this repository.
2. Copy `sync/.env.example` to `sync/.env`.
3. Build the sync worker image:

```bash
docker compose build
```

## Configure The Sync Worker

Copy the example environment file:

```bash
cp sync/.env.example sync/.env
```

Edit `sync/.env` and set the required values:

```env
SYNC_VAULT_NAME=your-vault-name-or-id
SYNC_VAULT_PASSWORD=your-vault-encryption-password
SYNC_OBSIDIAN_AUTH_TOKEN=your-obsidian-auth-token
```

Optional values:

```env
SYNC_DEVICE_NAME=canterbury-sync
SYNC_VAULT_PATH=/vault
```

| Variable                   | Required | Description                                                                |
| -------------------------- | -------- | -------------------------------------------------------------------------- |
| `SYNC_VAULT_NAME`          | Yes      | Remote Obsidian vault name or ID.                                          |
| `SYNC_VAULT_PASSWORD`      | Yes      | End-to-end encryption password for the remote vault.                       |
| `SYNC_OBSIDIAN_AUTH_TOKEN` | Yes      | Obsidian auth token used by `obsidian-headless`.                           |
| `SYNC_DEVICE_NAME`         | No       | Device name shown in Obsidian Sync history. Defaults to `canterbury-sync`. |
| `SYNC_VAULT_PATH`          | No       | Local vault path inside the container. Defaults to `/vault` in Compose.    |

Do not commit `sync/.env`. This repository ignores it because it contains
secrets.

## Run The Sync Worker

Start the sync worker:

```bash
docker compose up --build
```

Run it in the background:

```bash
docker compose up --build -d
```

Stop it:

```bash
docker compose down
```

The service uses the `obsidian-vault` Docker volume by default:

```yaml
volumes:
  - obsidian-vault:/vault
```

This is the portable default. If you replace it with a host bind mount such as
`./vault:/vault`, you must ensure the container user can write to that host
directory.

## Run The Vault Service

The vault service reads from a local filesystem mirror of the vault. If you use
the default Docker-managed `obsidian-vault` volume, that volume is not exposed as
`./vault` on the host. For local service development, point
`VAULT_SERVICE_ROOT` at a host-accessible mirror, or deliberately use a bind
mount override such as `./vault:/vault` after accounting for host filesystem
permissions.

Copy the vault service example environment file:

```bash
cp .env.example .env
```

Configure the vault service values:

```env
VAULT_SERVICE_ROOT=./vault
VAULT_SERVICE_AUTH_SCOPES=personal-agent
VAULT_SERVICE_ADDR=127.0.0.1:50051
```

| Variable                    | Required | Description                                                                   |
| --------------------------- | -------- | ----------------------------------------------------------------------------- |
| `VAULT_SERVICE_ROOT`        | Yes      | Local filesystem path to the mirrored vault read by the Go vault service.     |
| `VAULT_SERVICE_AUTH_SCOPES` | Yes      | Comma-separated principal scopes granted to the local vault service instance. |
| `VAULT_SERVICE_ADDR`        | No       | Address for the Connect server. Defaults to `127.0.0.1:50051` when not set.   |

The vault service loads `.env` from the repository root when present. Real
environment variables take precedence over values in `.env`.

Start the local service:

```bash
go run ./cmd/vault-service
```

The service exposes Connect/gRPC on the configured address. The current
implemented vault RPCs are:

```text
/canterbury.vault.v1.VaultService/ReadNote
/canterbury.vault.v1.VaultService/SearchNotes
```

Example `ReadNote` request body:

```json
{
	"ref": {
		"path": "Projects/Canterbury.md"
	}
}
```

`ReadNote` returns the Markdown body without frontmatter in `note.content`.
Parsed frontmatter is returned in `note.metadata.properties` after reserved
access-policy fields are removed. YAML timestamp values are formatted as RFC
3339 strings so they can be represented in `google.protobuf.Struct`.

Example `SearchNotes` request body:

```json
{
	"query": {
		"text": "Canterbury"
	},
	"filter": {
		"includePathPrefixes": ["Projects"],
		"excludePathPrefixes": ["Projects/Archive"],
		"allTags": ["project"],
		"anyTags": ["ai", "notes"]
	},
	"pageSize": 25,
	"sort": "SEARCH_SORT_PATH_ASC"
}
```

`SearchNotes` returns matching note references, metadata, snippets, and a
`nextPageToken` when another page is available. The current text query is a
single trimmed search term matched case-insensitively against note content. It
is not parsed as comma-separated syntax. Path prefix filters use vault-relative
paths. `all_tags` requires every listed tag, while `any_tags` requires at least
one listed tag when present. Supported sort orders are
`SEARCH_SORT_PATH_ASC` and `SEARCH_SORT_MODIFIED_DESC`.

The local service currently uses the fixed principal scopes from
`VAULT_SERVICE_AUTH_SCOPES`. It does not yet authenticate each request, emit
audit records, or expose MCP tools, so keep it bound to a trusted local
interface while it is in this development shape.

## Develop Canterbury

Install repository formatting dependencies:

```bash
npm install
```

Install Go linting tooling:

```bash
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.11.4
```

Install development dependencies for the sync component:

```bash
npm --prefix sync install
```

Run all checks:

```bash
npm run check
```

Format files from the repository root:

```bash
npm run format
```

Run Go tests:

```bash
npm run test:go
```

Run Go linting:

```bash
npm run lint:go
```

Lint sync worker files:

```bash
npm --prefix sync run lint
```

Auto-fix sync worker lint issues where possible:

```bash
npm --prefix sync run lint:fix
```

You can also run the sync component check directly:

```bash
npm --prefix sync run check
```

## Troubleshoot

| Problem                                                        | Cause                                                                         | Solution                                                                                                              |
| -------------------------------------------------------------- | ----------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `SYNC_VAULT_NAME is required`                                  | `sync/.env` is missing the vault name or ID.                                  | Set `SYNC_VAULT_NAME`.                                                                                                |
| `SYNC_VAULT_PASSWORD is required`                              | `sync/.env` is missing the vault encryption password.                         | Set `SYNC_VAULT_PASSWORD`.                                                                                            |
| `SYNC_OBSIDIAN_AUTH_TOKEN is required`                         | `sync/.env` is missing the Obsidian auth token.                               | Set `SYNC_OBSIDIAN_AUTH_TOKEN`.                                                                                       |
| `environment variable "VAULT_SERVICE_ROOT" is required`        | `.env` or the shell environment is missing the vault service root path.       | Set `VAULT_SERVICE_ROOT` to the mirrored vault path.                                                                  |
| `environment variable "VAULT_SERVICE_AUTH_SCOPES" is required` | `.env` or the shell environment is missing principal scopes for local access. | Set `VAULT_SERVICE_AUTH_SCOPES` to a comma-separated scope list such as `personal-agent`.                             |
| `permission denied; check your authorization scopes`           | The note does not declare a scope granted to the local vault service.         | Add a matching `access.scopes` value to the note or update `VAULT_SERVICE_AUTH_SCOPES` for local development.         |
| `invalid search query`                                         | A search request contains an unsupported sort or invalid page token.          | Use `SEARCH_SORT_PATH_ASC` or `SEARCH_SORT_MODIFIED_DESC`, and only reuse `nextPageToken` values returned by search.  |
| `Another sync instance is already running for this vault.`     | Another sync process owns the vault lock, or the vault path is not writable.  | Stop other sync clients for the same vault and confirm the container can write to `/vault`.                           |
| The container exits after a sync failure.                      | Compose is configured with `restart: 'no'`.                                   | Inspect the logs with `docker compose logs obsidian-sync`, fix the configuration, then run `docker compose up` again. |
| Files are hard to inspect on the host.                         | The default Docker volume stores the vault outside the repository.            | Use Docker volume tooling to inspect it, or deliberately configure a bind mount for local development.                |

## Roadmap

The planned MVP includes:

- Read-only vault access through MCP-compatible tools.
- Scope-based frontmatter access control.
- Vault sync through Obsidian Headless.
- Audit logging outside the vault.
- Basic indexing with SQLite.

Initial non-goals include:

- GraphQL APIs.
- Vector search.
- Complex permission inheritance.
- Autonomous write access.
- Multi-tenant support.

## Contributing

Before contributing, run:

```bash
npm run check
```

Keep the default deployment portable. Prefer Docker-managed volumes for runtime
storage, and treat host bind mounts as local development overrides.

## Additional Documentation

- [`obsidian-headless`](https://www.npmjs.com/package/obsidian-headless)
- [Obsidian Sync](https://obsidian.md/sync)
- [Docker Compose](https://docs.docker.com/compose/)
- [The Good Docs Project README template guide](https://www.thegooddocsproject.dev/template/readme)

## How To Get Help

Start by checking the sync worker logs:

```bash
docker compose logs obsidian-sync
```

For problems with Obsidian account access, vault encryption, or Sync service
behavior, consult Obsidian support resources. For problems with Canterbury, open
an issue or discussion in the repository where this project is hosted.

## Terms Of Use

This project is licensed as `UNLICENSED` in `sync/package.json`.

Obsidian, Obsidian Sync, and `obsidian-headless` are separate projects with their
own terms and licenses.
