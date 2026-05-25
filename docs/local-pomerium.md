# Local Pomerium Stack

Canterbury includes a default local Docker Compose stack that puts the vault
service behind Pomerium Core and uses Dex as a self-contained OpenID Connect
provider. This is the local precursor to a deployed edge setup: Pomerium
authenticates the caller, issues its own signed JWT assertion, rewrites that
assertion into `Authorization: Bearer <jwt>`, and the vault service validates it
with Pomerium's JWKS before applying Canterbury scope policy.

The stack is for development only. The committed Pomerium keys, TLS certificate,
Dex client secret, and Dex password hashes under `deploy/local-pomerium/` are
fixtures and must not be reused outside local testing.

## Start The Stack

Run the local stack from the repository root:

```bash
docker compose up --build
```

The default Compose services are:

| Service         | Purpose                           | Local URL                                   |
| --------------- | --------------------------------- | ------------------------------------------- |
| `dex`           | Local OIDC provider               | `http://dex.localhost.pomerium.io:5556/dex` |
| `pomerium`      | Local identity-aware proxy        | `https://vault.localhost.pomerium.io:8443`  |
| `vault-service` | Canterbury Connect/gRPC vault API | Internal only, proxied by Pomerium          |

The Pomerium route uses a self-signed local certificate for
`*.localhost.pomerium.io`. Browsers and command-line tools will warn unless you
trust the certificate or explicitly allow it for local testing.

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
`deploy/local-pomerium/dex-config.yaml`, choose a stable `userID`, sign in
through Pomerium once, then read Pomerium's `user` value from the authorize log
and add a matching `(issuer, subject)` entry to `sample-auth/scopes.toml`.

## Smoke Test

After the stack is running, run:

```bash
make smoke-pomerium
```

The smoke test obtains Dex ID tokens with the local password grant, sends those
tokens to Pomerium, and verifies that Pomerium forwards its own signed assertion
to the vault service. It checks:

- `agent@canterbury.local` can read `Projects/Canterbury.md`.
- `public@canterbury.local` can read `Public/Service Brief.md`.
- `public@canterbury.local` cannot read `Projects/Canterbury.md`.
- `unmapped@canterbury.local` is rejected during Canterbury authentication.
- Requests without a bearer token are rejected by the protected route.

This smoke test is intentionally not part of `make check` because it requires
Docker and external image pulls.

## Inspect Audit Events

The vault service writes local JSONL audit records to `./audit`. For a quick
look at authentication failures:

```bash
grep '"event_type":"auth.failed"' audit/*.jsonl
```

For read and authorization-denial events:

```bash
grep '"event_type":"vault.read' audit/*.jsonl
```

The unmapped Dex account should produce an `auth.failed` event with a denied
policy decision. The public reader trying to read a personal note should produce
a vault read denial rather than an authentication failure.
