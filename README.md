# Canterbury

Canterbury is an experimental system for connecting AI agents to an Obsidian
vault through a controlled service layer. The long-term goal is to let agents
read, search, and eventually write vault content while enforcing explicit access
policies and recording an independent audit trail of every interaction.

This repository currently implements a Dockerized sync worker, an early local Go
vault service with scoped `ReadNote` and `SearchNotes` RPCs, and a development
auth helper for minting local JWTs.

## Project Status

Canterbury is in early development. The current implementation includes:

- A Node-based sync worker container.
- `obsidian-headless` integration.
- Docker Compose configuration.
- A local Go vault service that reads and searches scoped Markdown notes from a
  filesystem vault mirror.
- Local Docker Compose stack with Dex, Pomerium Core, and the vault service for
  testing the deployed-style auth gateway flow.
- JWT-based request authentication with JWKS verification and TOML scope
  mappings.
- Scope-based authorization using note-declared access scopes and authenticated
  principals.
- Date-rotated JSONL audit logging for vault read and search attempts.
- Connect/gRPC health, reflection, `ReadNote`, and `SearchNotes` handlers.
- A development auth CLI that starts a local Connect/gRPC service for minting
  local JWTs and serving its public verification key as JWKS.
- Repository formatting, test, and linting tooling.

Planned or incomplete components include:

- MCP-compatible tools for AI agents.
- Indexing and plugin-style vault operations.
- Production identity-provider integration beyond the local Pomerium/Dex and
  development auth helpers.

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
agent-facing service because MCP tools, indexing, write workflows, and
production identity-provider integration are still planned.

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

The current repository implements the sync worker, the first vault service read
and search paths, JWT-authenticated local access, a local Pomerium/Dex gateway
stack, and filesystem JSONL audit logging for read, search, and authentication
failure events. MCP tools, indexing, and write workflows are not implemented
yet.

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
service interfaces. The current vault service validates signed bearer JWTs,
maps each token issuer and subject to Canterbury scopes from a TOML file outside
the vault, and authorizes note access from those mapped scopes.

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
3. Build the default local stack:

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
docker compose --profile sync up --build obsidian-sync
```

Run it in the background:

```bash
docker compose --profile sync up --build -d obsidian-sync
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

## Run The Local Pomerium Stack

The default Docker Compose stack starts a local deployed-style auth path:

- Dex as a self-contained OIDC provider.
- Pomerium Core as the identity-aware gateway.
- The Canterbury vault service behind Pomerium.

Start it from the repository root:

```bash
docker compose up --build
```

The protected vault route is available at:

```text
https://vault.localhost.pomerium.io:8443
```

The stack uses local-only fixture keys, a self-signed certificate, and static
Dex users. See [Local Pomerium Stack](docs/local-pomerium.md) for account
details, manual testing steps, and the opt-in smoke test:

```bash
make smoke-pomerium
```

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
VAULT_SERVICE_ROOT=./sample-vault
VAULT_SERVICE_AUTH_ISSUER=devauth.canterbury.local
VAULT_SERVICE_AUTH_AUDIENCE=canterbury.vault.local
VAULT_SERVICE_AUTH_JWKS_URL=http://127.0.0.1:50052/.well-known/jwks.json
VAULT_SERVICE_AUTH_MAPPING_FILE=./sample-auth/scopes.toml
VAULT_SERVICE_AUDIT_ROOT=./audit
VAULT_SERVICE_AUDIT_WRITER_ID=
VAULT_SERVICE_ADDR=127.0.0.1:50051
VAULT_SERVICE_AUDIT_HMAC_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=
```

| Variable                          | Required | Description                                                                                                                                           |
| --------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `VAULT_SERVICE_ROOT`              | Yes      | Local filesystem path to the mirrored vault read by the Go vault service.                                                                             |
| `VAULT_SERVICE_AUTH_ISSUER`       | Yes      | Expected JWT issuer.                                                                                                                                  |
| `VAULT_SERVICE_AUTH_AUDIENCE`     | Yes      | Expected JWT audience.                                                                                                                                |
| `VAULT_SERVICE_AUTH_JWKS_URL`     | Yes      | JWKS endpoint used to verify signed bearer JWTs.                                                                                                      |
| `VAULT_SERVICE_AUTH_MAPPING_FILE` | Yes      | TOML file mapping exact `(issuer, subject)` pairs to Canterbury scopes.                                                                               |
| `VAULT_SERVICE_AUDIT_ROOT`        | Yes      | Local filesystem directory where date-rotated JSONL audit logs are written outside the vault.                                                         |
| `VAULT_SERVICE_AUDIT_HMAC_KEY`    | Yes      | Base64-encoded HMAC key, at least 32 decoded bytes, used to hash remote client addresses before writing audit records.                                |
| `VAULT_SERVICE_AUDIT_WRITER_ID`   | No       | Optional filesystem-safe identifier included in audit filenames. When unset, the service generates one from hostname, PID, and a short random suffix. |
| `VAULT_SERVICE_ADDR`              | No       | Address for the Connect server. Defaults to `127.0.0.1:50051` when not set.                                                                           |

The `.env.example` file includes a development-only sample
`VAULT_SERVICE_AUDIT_HMAC_KEY` so the local demo starts without shell expansion
inside the dotenv file. Generate a real local or deployment key with:

```bash
openssl rand -base64 32
```

Keep this key stable if you need remote address hashes to remain comparable
across service restarts. Rotate it when old audit address correlations should no
longer be linkable.

Use `./sample-vault` for a quick local demo. Point `VAULT_SERVICE_ROOT` at your
own host-accessible vault mirror when testing with real synced content.

The included sample vault contains a few fake notes for exercising the current
read, search, tag, and authorization behavior:

| Path                         | Access scopes                   | Demo use                                     |
| ---------------------------- | ------------------------------- | -------------------------------------------- |
| `Projects/Canterbury.md`     | `personal-agent`                | Basic `ReadNote` and `SearchNotes` success.  |
| `Projects/Agent Research.md` | `personal-agent`                | Text and tag search over project notes.      |
| `Public/Service Brief.md`    | `personal-agent`, `public-site` | Multiple-scope access checks.                |
| `Private/Unscoped Draft.md`  | None                            | Default-deny and permission-denied behavior. |

The Bruno collection in `bruno/canterbury` includes sample requests that target
the sample vault defaults. Add an `Authorization: Bearer <token>` header using a
token minted by the development auth service before sending vault requests.

The included `sample-auth/scopes.toml` maps the development auth subject
`user_123` to `personal-agent`. Mint a local token with subject `user_123` and
audience `canterbury.vault.local` when using the sample configuration.

The vault service loads `.env` from the repository root when present. Real
environment variables take precedence over values in `.env`.

### Run The Local Auth Smoke Test

The auth smoke test is opt-in and is not part of `make check`. It starts both
local services, runs Bruno CLI assertions, and then tears the services down:

```bash
make smoke-auth
```

You can run the same flow through npm:

```bash
npm run smoke:auth
```

The runner starts `go run ./cmd/dev-auth serve` on `127.0.0.1:50052`, waits for
`/.well-known/jwks.json`, starts `go run ./cmd/vault-service` on
`127.0.0.1:50051`, writes audit logs to a temporary directory, and runs
`bruno/local-auth-smoke` with the `local` environment. The smoke collection uses
Connect JSON requests so the Bruno CLI can mint tokens, chain runtime variables,
call the vault service, and assert success and failure cases in one ordered run.

For manual Bruno testing, start the services as shown below, then open or run the
collections under `bruno/devauth`, `bruno/canterbury`, or
`bruno/local-auth-smoke`. When using `bruno/canterbury` directly, mint a token
for subject `user_123` and audience `canterbury.vault.local`, then set
`VaultToken` in the `local` environment before sending vault requests.

Start the development auth service first so the vault service can fetch its JWKS
during startup:

```bash
go run ./cmd/dev-auth serve
```

In another shell, start the local vault service:

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

Read and search attempts are written to
date-rotated JSONL audit logs under `VAULT_SERVICE_AUDIT_ROOT`; each service
process writes its own per-day file using `VAULT_SERVICE_AUDIT_WRITER_ID` or a
generated writer ID. If a required read or search audit record cannot be
written, the service fails instead of returning data. Authentication failures
also produce `auth.failed` audit events before the request is rejected. The
service does not yet expose MCP tools, so keep it bound to a trusted local
interface while it is in this development shape.

The Connect vault handler attaches request-scoped audit metadata to each vault
RPC. It accepts or generates an `X-Request-ID`, returns that request ID on
success and error responses, extracts W3C `traceparent` trace IDs when present,
and records a keyed HMAC-SHA256 hash of the remote client address rather than
the raw address.

## Run The Development Auth Service

The development auth CLI is a local helper for JWT-based authentication testing.
It starts a Connect/gRPC service with health and reflection enabled on
`127.0.0.1:50052`:

```bash
go run ./cmd/dev-auth serve
```

The `serve` command accepts flags for its local server address and JWT issuer:

```bash
go run ./cmd/dev-auth serve \
  --addr 127.0.0.1:50052 \
  --issuer devauth.canterbury.local
```

| Variable          | Flag       | Description                                                                                  |
| ----------------- | ---------- | -------------------------------------------------------------------------------------------- |
| `DEV_AUTH_ADDR`   | `--addr`   | Address for the development auth Connect server. Defaults to `127.0.0.1:50052` when not set. |
| `DEV_AUTH_ISSUER` | `--issuer` | Issuer claim placed in minted development JWTs. Defaults to `devauth.canterbury.local`.      |

When `.env` is present at the repository root, the development auth command loads
these values from it without overriding real environment variables. Command-line
flags take precedence over environment and `.env` values. Run
`go run ./cmd/dev-auth serve --help` to print serve flags.

The `MintToken` RPC creates an EdDSA-signed bearer JWT for requested claims:

```json
{
	"claims": {
		"subject": "user_123",
		"audiences": ["canterbury.vault.local"]
	},
	"options": {
		"ttlSeconds": 900
	}
}
```

Omit `options.ttlSeconds` or set it to `0` to use the service default. The
application rejects missing subjects, missing audiences, negative TTLs, and TTLs
above the current local development maximum. The service exposes its public
verification key as JWKS at `http://127.0.0.1:50052/.well-known/jwks.json` so
local verifier work can use the same JWKS shape planned for deployed auth. The
key is generated in memory when the service starts, so tokens minted before a
restart do not verify against the new endpoint response. The Bruno collection in
`bruno/devauth` points at the dev-auth default address.

## Develop Canterbury

The recommended setup path installs repository dependencies and project-local
Go tooling:

```bash
make setup
```

Run the full repository check through Make so the configured local toolchain is
used:

```bash
make check
```

You can also run individual checks directly when you already have the required
tools installed.

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

Run all npm-orchestrated checks:

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

| Problem                                                              | Cause                                                                           | Solution                                                                                                              |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `SYNC_VAULT_NAME is required`                                        | `sync/.env` is missing the vault name or ID.                                    | Set `SYNC_VAULT_NAME`.                                                                                                |
| `SYNC_VAULT_PASSWORD is required`                                    | `sync/.env` is missing the vault encryption password.                           | Set `SYNC_VAULT_PASSWORD`.                                                                                            |
| `SYNC_OBSIDIAN_AUTH_TOKEN is required`                               | `sync/.env` is missing the Obsidian auth token.                                 | Set `SYNC_OBSIDIAN_AUTH_TOKEN`.                                                                                       |
| `environment variable "VAULT_SERVICE_ROOT" is required`              | `.env` or the shell environment is missing the vault service root path.         | Set `VAULT_SERVICE_ROOT` to `./sample-vault` for the demo or to your mirrored vault path.                             |
| `environment variable "VAULT_SERVICE_AUTH_ISSUER" is required`       | `.env` or the shell environment is missing the expected JWT issuer.             | Set `VAULT_SERVICE_AUTH_ISSUER` to the issuer used by your auth provider or local dev-auth service.                   |
| `environment variable "VAULT_SERVICE_AUTH_AUDIENCE" is required`     | `.env` or the shell environment is missing the expected JWT audience.           | Set `VAULT_SERVICE_AUTH_AUDIENCE`, and mint/request tokens with the same audience.                                    |
| `environment variable "VAULT_SERVICE_AUTH_JWKS_URL" is required`     | `.env` or the shell environment is missing the JWKS URL.                        | Set `VAULT_SERVICE_AUTH_JWKS_URL`, such as `http://127.0.0.1:50052/.well-known/jwks.json` for local development.      |
| `environment variable "VAULT_SERVICE_AUTH_MAPPING_FILE" is required` | `.env` or the shell environment is missing the scope mapping path.              | Set `VAULT_SERVICE_AUTH_MAPPING_FILE` to a TOML mapping file such as `./sample-auth/scopes.toml`.                     |
| `environment variable "VAULT_SERVICE_AUDIT_ROOT" is required`        | `.env` or the shell environment is missing the audit log directory.             | Set `VAULT_SERVICE_AUDIT_ROOT` to an explicit directory outside the vault, such as `./audit`.                         |
| `environment variable "VAULT_SERVICE_AUDIT_HMAC_KEY" is required`    | `.env` or the shell environment is missing the audit HMAC key.                  | Generate a literal key with `openssl rand -base64 32` and set `VAULT_SERVICE_AUDIT_HMAC_KEY`.                         |
| `parse audit hmac key`                                               | `VAULT_SERVICE_AUDIT_HMAC_KEY` is not base64 or decodes to fewer than 32 bytes. | Replace it with a literal key generated by `openssl rand -base64 32`.                                                 |
| `permission denied; check your authorization scopes`                 | The note does not declare a scope mapped to the authenticated subject.          | Add a matching `access.scopes` value to the note or update the auth mapping file for local development.               |
| `invalid search query`                                               | A search request contains an unsupported sort or invalid page token.            | Use `SEARCH_SORT_PATH_ASC` or `SEARCH_SORT_MODIFIED_DESC`, and only reuse `nextPageToken` values returned by search.  |
| `Another sync instance is already running for this vault.`           | Another sync process owns the vault lock, or the vault path is not writable.    | Stop other sync clients for the same vault and confirm the container can write to `/vault`.                           |
| The container exits after a sync failure.                            | Compose is configured with `restart: 'no'`.                                     | Inspect the logs with `docker compose logs obsidian-sync`, fix the configuration, then run `docker compose up` again. |
| Files are hard to inspect on the host.                               | The default Docker volume stores the vault outside the repository.              | Use Docker volume tooling to inspect it, or deliberately configure a bind mount for local development.                |

## Roadmap

The planned MVP includes:

- Read-only vault access through MCP-compatible tools.
- Scope-based frontmatter access control.
- Production-ready authentication integrations and more robust principal-to-scope policy management.
- Hardening and operationalizing vault sync through Obsidian Headless.
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
- [Local Pomerium Stack](docs/local-pomerium.md)
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
