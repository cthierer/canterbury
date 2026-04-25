# Canterbury

Canterbury is an experimental system for connecting AI agents to an Obsidian
vault through a controlled service layer. The long-term goal is to let agents
read, search, and eventually write vault content while enforcing explicit access
policies and recording an independent audit trail of every interaction.

This repository currently implements the first component: a Dockerized sync
worker that mirrors an Obsidian Sync vault into managed container storage.

## Project Status

Canterbury is in early development. The current implementation includes:

- A Node-based sync worker container.
- `obsidian-headless` integration.
- Docker Compose configuration.
- Formatting and linting tooling for the sync component.

Planned components include:

- A Go vault service that exposes controlled vault access.
- MCP-compatible tools for AI agents.
- Frontmatter-based authorization and classification.
- Independent audit logging.
- Indexing and plugin-style vault operations.

## Project Description

Canterbury aims to act as secure middleware between AI agents and an Obsidian
vault. AI agents should not read or write vault files directly. Instead, agents
will interact with a service that authorizes requests, exposes structured query
interfaces, and records actions outside the vault.

The sync worker is the trusted component that mirrors the vault. Future services
will consume that mirrored vault without receiving Obsidian account credentials.

## Who This Project Is For

Use Canterbury if you want to explore AI-assisted workflows over an Obsidian
vault while keeping clear boundaries around access, authorization, and audit
records.

The current sync worker is useful if you want to run Obsidian Sync in Docker on
a server, NAS, homelab machine, or cloud container environment.

Canterbury is not a replacement for an Obsidian Sync subscription. The sync
worker requires valid Obsidian Sync credentials and an existing remote vault.

## Architecture

The intended system has several components:

- **Sync worker**: Mirrors an Obsidian vault using `obsidian-headless`.
- **Vault service**: Provides the primary controlled interface to vault data.
- **Authorization and classification**: Grants access only to explicitly marked
  files and matching agent scopes.
- **Audit system**: Records operations outside the vault in an append-only log.
- **Indexing layer**: Parses notes into searchable metadata.
- **Plugin and operation framework**: Runs extensible processing over vault
  events and content.

The current repository only implements the sync worker.

## Access Model

The planned access model is default deny. Notes will opt in to AI access through
frontmatter such as:

```yaml
ai:
  access: true
  scopes:
    - personal-agent
```

Agents will operate with scoped identities. A future vault service will grant
access only when a note explicitly allows AI access and the agent scope matches
the note scope.

## Project Dependencies

To run the current sync worker, you need:

- Docker with Docker Compose.
- An Obsidian account with Sync enabled.
- An Obsidian auth token.
- The name or ID of the remote vault to sync.
- The end-to-end encryption password for the remote vault.

To develop the sync component, you also need:

- Node.js 24 or a compatible version supported by the sync dependencies.
- npm.

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

## Develop Canterbury

Install development dependencies for the sync component:

```bash
cd sync
npm install
```

Run all checks:

```bash
npm run check
```

Format files:

```bash
npm run format
```

Lint files:

```bash
npm run lint
```

Auto-fix lint issues where possible:

```bash
npm run lint:fix
```

## Troubleshoot The Sync Worker

| Problem                                                    | Cause                                                                        | Solution                                                                                                              |
| ---------------------------------------------------------- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `SYNC_VAULT_NAME is required`                              | `sync/.env` is missing the vault name or ID.                                 | Set `SYNC_VAULT_NAME`.                                                                                                |
| `SYNC_VAULT_PASSWORD is required`                          | `sync/.env` is missing the vault encryption password.                        | Set `SYNC_VAULT_PASSWORD`.                                                                                            |
| `SYNC_OBSIDIAN_AUTH_TOKEN is required`                     | `sync/.env` is missing the Obsidian auth token.                              | Set `SYNC_OBSIDIAN_AUTH_TOKEN`.                                                                                       |
| `Another sync instance is already running for this vault.` | Another sync process owns the vault lock, or the vault path is not writable. | Stop other sync clients for the same vault and confirm the container can write to `/vault`.                           |
| The container exits after a sync failure.                  | Compose is configured with `restart: 'no'`.                                  | Inspect the logs with `docker compose logs obsidian-sync`, fix the configuration, then run `docker compose up` again. |
| Files are hard to inspect on the host.                     | The default Docker volume stores the vault outside the repository.           | Use Docker volume tooling to inspect it, or deliberately configure a bind mount for local development.                |

## Roadmap

The planned MVP includes:

- Read-only vault access through MCP-compatible tools.
- Frontmatter-based access control.
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

Before contributing to the sync component, run:

```bash
cd sync
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
