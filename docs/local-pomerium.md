# Local Pomerium Stack

Canterbury includes a default local Docker Compose stack that puts the vault and
MCP services behind Pomerium Core and uses Dex as a self-contained OpenID
Connect provider. This is the local precursor to a deployed edge setup:
Pomerium authenticates the caller, issues its own signed JWT assertion, rewrites
that assertion into `Authorization: Bearer <jwt>`, and Canterbury validates it
before applying vault scope policy.

The stack is for development only. Pomerium private keys, the TLS certificate,
the Dex client secret, and shared secrets are generated locally and must not be
reused outside local testing.

## Start The Stack

Generate the local-only Pomerium config, certificates, keys, and shared secrets
first:

```bash
scripts/setup-local-pomerium.mjs
```

The setup script writes generated files under
`deploy/local-pomerium/.generated/` and secrets to
`deploy/local-pomerium/local.env`. Both paths are ignored by Git. Re-run the
script any time you need to recreate the local environment in one step.

Run the local stack from the repository root:

```bash
CANTERBURY_UID="$(id -u)" CANTERBURY_GID="$(id -g)" docker compose up --build
```

The `CANTERBURY_UID` and `CANTERBURY_GID` values make the vault-service
container write local audit files as your host user. If your host user and group
IDs are both `1000`, Docker Compose's default interpolation matches this value,
but exporting the values keeps the bind-mounted `./audit` directory predictable
on other systems.

The default Compose services are:

| Service         | Purpose                           | Local URL                                      |
| --------------- | --------------------------------- | ---------------------------------------------- |
| `dex`           | Local OIDC provider               | `http://dex.localhost.pomerium.io:5556/dex`    |
| `pomerium`      | Local identity-aware proxy        | `https://vault.localhost.pomerium.io:8443`     |
| `vault-service` | Canterbury Connect/gRPC vault API | Internal only, proxied by Pomerium             |
| `mcp-server`    | Stateless Streamable HTTP MCP API | `https://vault.localhost.pomerium.io:8443/mcp` |

The MCP container does not publish its listener to the host. The `/mcp` route is
matched before the catch-all vault route and forwards to the internal MCP
service. Both routes use the same external hostname so Pomerium's assertion
issuer and audience remain acceptable to the vault service when the MCP server
forwards the assertion.

The route is an ordinary protected HTTP route, not Pomerium's experimental
MCP-native OAuth mode. MCP clients must send a bearer credential that the
existing Pomerium route accepts.

The Pomerium route uses a self-signed local certificate for
`*.localhost.pomerium.io`. Browsers and command-line tools will warn unless you
trust `deploy/local-pomerium/.generated/certs/pomerium-local.crt` or explicitly
allow it for local testing.

The sync worker is no longer part of the default stack. To run it, use the
explicit `sync` profile:

```bash
docker compose --profile sync up obsidian-sync
```

The sync worker still uses the portable Docker-managed named volume:

```yaml
volumes:
  - obsidian-vault:/vault
```

## Test Accounts

Dex stores three static users for local testing. Every account uses password
`password`.

| Email                       | Dex `userID`    | Pomerium `sub`                   | Canterbury scopes | Expected behavior                                  |
| --------------------------- | --------------- | -------------------------------- | ----------------- | -------------------------------------------------- |
| `agent@canterbury.local`    | `user_123`      | `Cgh1c2VyXzEyMxIFbG9jYWw`        | `personal-agent`  | Can read personal-agent sample notes.              |
| `public@canterbury.local`   | `public_reader` | `Cg1wdWJsaWNfcmVhZGVyEgVsb2NhbA` | `public-site`     | Can read public-site notes, not personal notes.    |
| `unmapped@canterbury.local` | `unmapped_user` | `Cg11bm1hcHBlZF91c2VyEgVsb2NhbA` | None              | Authenticates at Pomerium, rejected by Canterbury. |

Canterbury maps Pomerium subjects in `sample-auth/scopes.toml`. For this Dex
setup, Pomerium derives the assertion `sub` from the Dex `userID` and the local
Dex connector. Use Pomerium's stable `sub`, not the email address:

```toml
[[subjects]]
issuer = "vault.localhost.pomerium.io"
subject = "Cgh1c2VyXzEyMxIFbG9jYWw"
scopes = ["personal-agent"]
```

To add a local account, add a `staticPasswords` entry in
`deploy/local-pomerium/dex-config.template.yaml`, re-run
`scripts/setup-local-pomerium.mjs`, choose a stable `userID`, sign in through
Pomerium once, then read Pomerium's `user` value from the authorize log and add
a matching `(issuer, subject)` entry to `sample-auth/scopes.toml`.

## Smoke Test

After the stack is running, run:

```bash
make smoke-pomerium
```

The smoke test obtains Dex ID tokens with the local password grant, sends those
tokens to Pomerium, and verifies that Pomerium forwards its own signed assertion
through both the Connect and MCP routes. It checks:

- `agent@canterbury.local` can read `Projects/Canterbury.md`.
- `public@canterbury.local` can read `Public/Service Brief.md`.
- `public@canterbury.local` cannot read `Projects/Canterbury.md`.
- `unmapped@canterbury.local` is rejected during Canterbury authentication.
- Requests without a bearer token are rejected by the protected route.
- `/mcp` lists exactly `read_note` and `search_notes`.
- MCP read and search calls enforce the same three identities and note scopes.
- MCP calls produce the underlying mandatory vault read and search audit events.

This smoke test is intentionally not part of `make check` because it requires
Docker and external image pulls.

## Inspect Audit Events

The vault service writes local JSONL audit records to `./audit`. For a quick
look at authentication failures:

```bash
find audit -type f -name '*.jsonl' -exec grep -h '"event_type":"auth.failed"' {} +
```

For read and authorization-denial events:

```bash
find audit -type f -name '*.jsonl' -exec grep -h '"event_type":"vault.read' {} +
```

The unmapped Dex account should produce an `auth.failed` event with a denied
policy decision. The public reader trying to read a personal note should produce
a vault read denial rather than an authentication failure.
